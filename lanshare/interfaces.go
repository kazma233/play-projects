package main

// Storage 存储接口
type Storage interface {
	// 数据操作
	Add(item *Item)
	Remove(id string)
	Get(id string) *Item
	List() []*Item
	Len() int

	// 生命周期
	Close() error

	// 维护操作
	Compact() error
	CleanupExpired(maxAge int64) error
}
