package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type MemoryStorage struct {
	items        map[string]*Item
	storageFile  *os.File
	lock         sync.RWMutex
	autoSaveChan chan ItemC
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewMemoryStorage(storagePath string) (*MemoryStorage, error) {
	fp := filepath.Join(storagePath, "metadata.jsonl")
	log.Printf("MemoryStorage path %s", fp)

	dirPath := filepath.Dir(fp)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &MemoryStorage{
		items:        make(map[string]*Item),
		storageFile:  f,
		autoSaveChan: make(chan ItemC, 100),
		ctx:          ctx,
		cancel:       cancel,
	}

	s.wg.Add(1)
	go s.autoSaveWorker()

	return s, nil
}

func (s *MemoryStorage) autoSaveWorker() {
	defer s.wg.Done()

	saveTicker := time.NewTicker(3 * time.Second)
	defer saveTicker.Stop()

	// 每天清理一次过期数据
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	// 每小时 compact 一次 JSONL 文件
	compactTicker := time.NewTicker(1 * time.Hour)
	defer compactTicker.Stop()

	for {
		select {
		// 处理剩余的保存任务
		case <-s.ctx.Done():
			for {
				select {
				case ic := <-s.autoSaveChan:
					log.Printf("Auto done save: op=%d, item_id=%s", ic.OP, ic.Item.ID)
					if err := s.persistItem(ic); err != nil {
						log.Printf("Failed to save item during shutdown: %v", err)
					}
				default:
					return
				}
			}
		case ic := <-s.autoSaveChan:
			log.Printf("Auto save: op=%d, item_id=%s", ic.OP, ic.Item.ID)
			if err := s.persistItem(ic); err != nil {
				log.Printf("Auto save error: %v", err)
			}
		case <-saveTicker.C:
			log.Println("sync file")
			// 定期同步文件
			if err := s.storageFile.Sync(); err != nil {
				log.Printf("File sync error: %v", err)
			}
		case <-cleanupTicker.C:
			// 定期清理30天前的数据
			if err := s.CleanupExpired(30); err != nil {
				log.Printf("Cleanup expired failed: %v", err)
			}
		case <-compactTicker.C:
			// 定期压缩 JSONL 文件
			if err := s.Compact(); err != nil {
				log.Printf("Compact failed: %v", err)
			}
		}
	}
}

func (s *MemoryStorage) persistItem(ic ItemC) error {
	log.Printf("persistItem: op=%d, item_id=%s, file=%s", ic.OP, ic.Item.ID, s.storageFile.Name())

	// 打印写入前文件状态
	beforeOffset, _ := s.storageFile.Seek(0, 1)
	beforeSize := s.getFileSize()
	log.Printf("persistItem: before offset=%d, size=%d", beforeOffset, beforeSize)

	data := map[string]interface{}{
		"op":   ic.OP,
		"item": ic.Item,
		"ts":   time.Now().Unix(),
	}

	encoder := json.NewEncoder(s.storageFile)
	err := encoder.Encode(data)
	if err != nil {
		log.Printf("persistItem encode error: %v", err)
		return err
	}

	// 立即 Sync 确保数据写入磁盘
	if err := s.storageFile.Sync(); err != nil {
		log.Printf("persistItem sync error: %v", err)
	}

	// 获取当前文件偏移量
	offset, _ := s.storageFile.Seek(0, 1)
	log.Printf("persistItem success: op=%d, item_id=%s, file_offset=%d, actual_size=%d", ic.OP, ic.Item.ID, offset, s.getFileSize())
	return nil
}

func (s *MemoryStorage) Close() error {
	log.Printf("Close: file=%s, size=%d", s.storageFile.Name(), s.getFileSize())
	s.cancel()
	s.wg.Wait()
	return s.storageFile.Close()
}

// 添加清理过期数据的方法
// 实现 IDataOperator 接口
func (s *MemoryStorage) Add(item *Item) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.items[item.ID] = item

	// 非阻塞发送，避免死锁
	select {
	case s.autoSaveChan <- ItemC{Item: *item, OP: ADD}:
	default:
		log.Printf("Warning: autoSaveChan full, ADD operation may be lost")
	}
}

func (s *MemoryStorage) Remove(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.items, id)

	// 非阻塞发送，避免死锁
	select {
	case s.autoSaveChan <- ItemC{Item: Item{ID: id}, OP: REMOVE}:
	default:
		log.Printf("Warning: autoSaveChan full, REMOVE operation may be lost")
	}
}

func (s *MemoryStorage) Get(id string) *Item {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.items[id]
}

func (s *MemoryStorage) List() []*Item {
	s.lock.RLock()
	defer s.lock.RUnlock()

	items := make([]*Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}

	// 按创建时间倒序排序（最新的在前面）
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreateTime.After(items[j].CreateTime)
	})

	return items
}

func (s *MemoryStorage) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return len(s.items)
}

func (s *MemoryStorage) getFileSize() int64 {
	info, err := s.storageFile.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// 实现 IMemoryStorage 接口
func (s *MemoryStorage) Load() error {
	log.Printf("Load: file=%s, size_before=%d", s.storageFile.Name(), s.getFileSize())
	s.lock.Lock()
	defer s.lock.Unlock()

	// 重置文件指针到开头
	_, err := s.storageFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	decoder := json.NewDecoder(s.storageFile)

	for {
		var data map[string]interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Failed to decode line: %v", err)
			continue
		}

		opFloat, ok := data["op"].(float64)
		if !ok {
			continue
		}
		op := OP(opFloat)

		itemData, ok := data["item"].(map[string]interface{})
		if !ok {
			continue
		}

		switch op {
		case ADD:
			// 完整解析 Item
			itemBytes, err := json.Marshal(itemData)
			if err != nil {
				continue
			}
			var item Item
			if err := json.Unmarshal(itemBytes, &item); err != nil {
				continue
			}
			s.items[item.ID] = &item

		case REMOVE:
			// REMOVE 操作只需要 ID，不需要完整解析 Item（避免 time.Time 零值问题）
			if id, ok := itemData["id"].(string); ok {
				delete(s.items, id)
			}
		}
	}

	// 读取完成后，重置文件指针到开头
	if _, err := s.storageFile.Seek(0, 0); err != nil {
		log.Printf("Load: failed to reset file pointer: %v", err)
	}
	currentPos, _ := s.storageFile.Seek(0, 1)
	log.Printf("Load: done, items_loaded=%d, file_pos_after=%d, size=%d", len(s.items), currentPos, s.getFileSize())
	return nil
}

// CleanupExpired 清理过期数据：从 JSONL 文件中删除过期项，并清理物理文件
// maxAge 单位为天
func (s *MemoryStorage) CleanupExpired(maxAge int64) error {
	log.Printf("CleanupExpired: file=%s, size=%d", s.storageFile.Name(), s.getFileSize())
	s.lock.Lock()
	defer s.lock.Unlock()

	fileName := s.storageFile.Name()

	// 读取原始文件
	_, err := s.storageFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	decoder := json.NewDecoder(s.storageFile)

	cutoff := time.Now().AddDate(0, 0, -int(maxAge))
	// 收集所有未过期的有效数据
	validItems := make(map[string]*Item)
	var expiredCount int
	var fileDeletedCount int

	for {
		var data map[string]interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Failed to decode line: %v", err)
			continue
		}

		opFloat, ok := data["op"].(float64)
		if !ok {
			continue
		}
		op := OP(opFloat)

		itemData, ok := data["item"].(map[string]interface{})
		if !ok {
			continue
		}

		itemBytes, err := json.Marshal(itemData)
		if err != nil {
			continue
		}

		var item Item
		if err := json.Unmarshal(itemBytes, &item); err != nil {
			continue
		}

		switch op {
		case ADD:
			if item.CreateTime.Before(cutoff) {
				// 过期了，需要删除物理文件
				expiredCount++
				if item.Type != TEXT {
					if saveName, ok := item.Meta["saveName"].(string); ok {
						if path, ok := item.Meta["path"].(string); ok {
							filePath := filepath.Join(path, saveName)
							if err := os.Remove(filePath); err != nil {
								log.Printf("Failed to delete expired file %s: %v", filePath, err)
							} else {
								fileDeletedCount++
								log.Printf("Deleted expired file: %s", filePath)
							}
						}
					}
				}
			} else {
				validItems[item.ID] = &item
			}
		case REMOVE:
			delete(validItems, item.ID)
		}
	}

	// 如果没有有效数据，直接清空文件
	if len(validItems) == 0 {
		if err := s.storageFile.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate file: %w", err)
		}
		if _, err := s.storageFile.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek file: %w", err)
		}
		if expiredCount > 0 {
			log.Printf("CleanupExpired: cleared all data, %d expired items removed", expiredCount)
		}
		return nil
	}

	// 创建临时文件
	tempFilePath := fileName + ".tmp"
	tempFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// 重新写入所有未过期的有效数据
	encoder := json.NewEncoder(tempFile)
	for _, item := range validItems {
		data := map[string]interface{}{
			"op":   ADD,
			"item": item,
			"ts":   time.Now().Unix(),
		}
		if err := encoder.Encode(data); err != nil {
			os.Remove(tempFilePath)
			return fmt.Errorf("failed to encode item: %w", err)
		}
	}

	// 同步临时文件
	if err := tempFile.Sync(); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	tempFile.Close()

	// 重命名临时文件（原子操作）
	if err := os.Rename(tempFilePath, fileName); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// 重置主文件指针
	if _, err := s.storageFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file after cleanup: %w", err)
	}

	if expiredCount > 0 {
		log.Printf("CleanupExpired: %d expired items removed, %d physical files deleted, %d items retained, new_size=%d",
			expiredCount, fileDeletedCount, len(validItems), s.getFileSize())
	}

	return nil
}

// Compact 从 JSONL 文件中删除 REMOVE 操作的记录，只保留有效的 ADD 操作
func (s *MemoryStorage) Compact() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	fileName := s.storageFile.Name()

	// 获取文件信息
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	log.Printf("Compact: file=%s, size=%d", fileName, fileInfo.Size())

	// 如果文件很小(<1KB)，不需要 compact
	if fileInfo.Size() < 1024 {
		log.Printf("Compact: skipped (size < 1024)")
		return nil
	}

	// 读取原始文件
	_, err = s.storageFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	decoder := json.NewDecoder(s.storageFile)

	// 收集所有有效的 ADD 操作（去重，保留最后一条）
	validItems := make(map[string]*Item)
	var removedCount int

	for {
		var data map[string]interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Failed to decode line: %v", err)
			continue
		}

		opFloat, ok := data["op"].(float64)
		if !ok {
			continue
		}
		op := OP(opFloat)

		itemData, ok := data["item"].(map[string]interface{})
		if !ok {
			continue
		}

		itemBytes, err := json.Marshal(itemData)
		if err != nil {
			continue
		}

		var item Item
		if err := json.Unmarshal(itemBytes, &item); err != nil {
			continue
		}

		switch op {
		case ADD:
			validItems[item.ID] = &item
		case REMOVE:
			if _, existed := validItems[item.ID]; existed {
				removedCount++
			}
			delete(validItems, item.ID)
		}
	}

	// 如果没有有效数据，直接清空文件
	if len(validItems) == 0 {
		if err := s.storageFile.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate file: %w", err)
		}
		if _, err := s.storageFile.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek file: %w", err)
		}
		log.Println("Compacted storage file: cleared all data")
		return nil
	}

	// 创建临时文件
	tempFilePath := fileName + ".tmp"
	tempFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// 重新写入所有有效数据
	encoder := json.NewEncoder(tempFile)
	for _, item := range validItems {
		data := map[string]interface{}{
			"op":   ADD,
			"item": item,
			"ts":   time.Now().Unix(),
		}
		if err := encoder.Encode(data); err != nil {
			os.Remove(tempFilePath)
			return fmt.Errorf("failed to encode item: %w", err)
		}
	}

	// 同步临时文件
	if err := tempFile.Sync(); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	tempFile.Close()

	// 重命名临时文件（原子操作）
	if err := os.Rename(tempFilePath, fileName); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// 重置主文件指针
	if _, err := s.storageFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file after compact: %w", err)
	}

	log.Printf("Compacted storage file: %d items retained, %d removed, new_size=%d", len(validItems), removedCount, s.getFileSize())
	return nil
}
