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
	return s.appendRecord(ic.OP, &ic.Item)
}

func (s *MemoryStorage) Close() error {
	log.Printf("Close: file=%s, size=%d", s.storageFile.Name(), s.getFileSize())
	s.cancel()
	s.wg.Wait()
	return s.storageFile.Close()
}

// deletePhysicalFile 删除物理文件，成功返回 true
func (s *MemoryStorage) deletePhysicalFile(item *Item) bool {
	if item.Type == TEXT {
		return false
	}

	saveName, ok1 := item.Meta["saveName"].(string)
	path, ok2 := item.Meta["path"].(string)
	if !ok1 || !ok2 {
		return false
	}

	filePath := filepath.Join(path, saveName)
	if err := os.Remove(filePath); err != nil {
		log.Printf("Failed to delete physical file %s: %v", filePath, err)
		return false
	}

	log.Printf("Deleted physical file: %s", filePath)
	return true
}

// 添加清理过期数据的方法
// 实现 IDataOperator 接口
func (s *MemoryStorage) Add(item *Item) {
	s.lock.Lock()
	s.items[item.ID] = item
	s.lock.Unlock()

	s.sendToWorker(ItemC{Item: *item, OP: ADD}, "ADD")
}

func (s *MemoryStorage) Remove(id string) {
	s.lock.Lock()
	delete(s.items, id)
	s.lock.Unlock()

	s.sendToWorker(ItemC{Item: Item{ID: id}, OP: REMOVE}, "REMOVE")
}

// sendToWorker 带超时阻塞发送给 worker
func (s *MemoryStorage) sendToWorker(ic ItemC, opName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case s.autoSaveChan <- ic:
	case <-ctx.Done():
		log.Printf("Warning: persist timeout for %s item %s, data may be lost on restart", opName, ic.Item.ID)
	case <-s.ctx.Done():
		// 程序正在关闭，直接返回（内存已更新）
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

// getFileSize 获取当前文件大小
func (s *MemoryStorage) getFileSize() int64 {
	info, err := s.storageFile.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// resetFilePointer 重置文件指针到文件开头
func (s *MemoryStorage) resetFilePointer() error {
	_, err := s.storageFile.Seek(0, 0)
	return err
}

// truncateFile 清空文件内容
func (s *MemoryStorage) truncateFile() error {
	if err := s.storageFile.Truncate(0); err != nil {
		return err
	}
	return s.resetFilePointer()
}

// reopenFile 关闭当前文件句柄并重新打开文件
func (s *MemoryStorage) reopenFile() error {
	fileName := s.storageFile.Name()
	s.storageFile.Close()
	newFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to reopen file: %w", err)
	}
	s.storageFile = newFile
	return nil
}

// readAllRecords 读取所有记录并重建状态
// 返回从文件中解析出的 items map（基于 op 操作重建的最终状态）
func (s *MemoryStorage) readAllRecords() (map[string]*Item, error) {
	if err := s.resetFilePointer(); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	items := make(map[string]*Item)
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
			itemBytes, err := json.Marshal(itemData)
			if err != nil {
				continue
			}
			var item Item
			if err := json.Unmarshal(itemBytes, &item); err != nil {
				continue
			}
			items[item.ID] = &item

		case REMOVE:
			if id, ok := itemData["id"].(string); ok {
				delete(items, id)
			}
		}
	}

	return items, nil
}

// atomicReplace 原子替换文件内容
func (s *MemoryStorage) atomicReplace(items map[string]*Item) error {
	fileName := s.storageFile.Name()
	tempFilePath := fileName + ".tmp"

	// 创建临时文件
	tempFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFilePath)

	// 写入临时文件
	if err := writeRecordsToFile(tempFile, items); err != nil {
		tempFile.Close()
		return err
	}

	// 同步并关闭临时文件
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	tempFile.Close()

	// 原子替换
	if err := os.Rename(tempFilePath, fileName); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// 重新打开文件
	return s.reopenFile()
}

// writeRecordsToFile 将记录写入指定文件
func writeRecordsToFile(f *os.File, items map[string]*Item) error {
	encoder := json.NewEncoder(f)
	for _, item := range items {
		data := map[string]interface{}{
			"op":   ADD,
			"item": item,
			"ts":   time.Now().Unix(),
		}
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("failed to encode item: %w", err)
		}
	}
	return nil
}

// appendRecord 追加单条记录到文件
func (s *MemoryStorage) appendRecord(op OP, item *Item) error {
	data := map[string]interface{}{
		"op":   op,
		"item": item,
		"ts":   time.Now().Unix(),
	}

	encoder := json.NewEncoder(s.storageFile)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode record: %w", err)
	}

	return s.storageFile.Sync()
}

func (s *MemoryStorage) Load() error {
	log.Printf("Load: file=%s, size=%d", s.storageFile.Name(), s.getFileSize())
	s.lock.Lock()
	defer s.lock.Unlock()

	items, err := s.readAllRecords()
	if err != nil {
		return err
	}

	s.items = items
	log.Printf("Load: done, items_loaded=%d", len(s.items))
	return nil
}

// CleanupExpired 清理过期数据：从 JSONL 文件中删除过期项，并清理物理文件
// maxAge 单位为天
func (s *MemoryStorage) CleanupExpired(maxAge int64) error {
	log.Printf("CleanupExpired: file=%s, size=%d", s.storageFile.Name(), s.getFileSize())
	s.lock.Lock()
	defer s.lock.Unlock()

	items, err := s.readAllRecords()
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -int(maxAge))
	validItems := make(map[string]*Item)
	var expiredCount, fileDeletedCount int

	for _, item := range items {
		if item.CreateTime.Before(cutoff) {
			// 过期了，需要删除物理文件
			expiredCount++
			if s.deletePhysicalFile(item) {
				fileDeletedCount++
			}
		} else {
			validItems[item.ID] = item
		}
	}

	// 如果没有有效数据，直接清空文件
	if len(validItems) == 0 {
		if err := s.truncateFile(); err != nil {
			return fmt.Errorf("failed to truncate file: %w", err)
		}
		log.Printf("CleanupExpired: cleared all data, %d expired items removed", expiredCount)
		return nil
	}

	// 原子替换文件
	if err := s.atomicReplace(validItems); err != nil {
		return err
	}

	log.Printf("CleanupExpired: %d expired items, %d files deleted, %d retained",
		expiredCount, fileDeletedCount, len(validItems))
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

	// 读取所有记录
	validItems, err := s.readAllRecords()
	if err != nil {
		return err
	}

	// 如果没有有效数据，直接清空文件
	if len(validItems) == 0 {
		if err := s.truncateFile(); err != nil {
			return fmt.Errorf("failed to truncate file: %w", err)
		}
		log.Println("Compacted storage file: cleared all data")
		return nil
	}

	// 原子替换文件
	if err := s.atomicReplace(validItems); err != nil {
		return err
	}

	log.Printf("Compacted storage file: %d items retained, new_size=%d", len(validItems), s.getFileSize())
	return nil
}
