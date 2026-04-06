package state

import (
	"fmt"
	"sync"
	"time"
)

// PrintJob хранит состояние задания печати пока пользователь настраивает параметры.
type PrintJob struct {
	FileID    string // Telegram file_id
	FileName  string // Имя файла
	FileType  string // "pdf", "jpg", "png"
	ChatID    int64  // ID чата
	MessageID int    // ID сообщения с inline-кнопками
	TempPath  string // Путь к скачанному файлу во /tmp

	// Параметры печати
	Copies   int
	Scale    int // % от страницы: 25, 50, 75, 100
	Rotation int // 0, 90, 180, 270

	// Размеры изображения (только для фото)
	ImgWidth  int
	ImgHeight int

	CreatedAt time.Time
}

func (j *PrintJob) IsImage() bool {
	return j.FileType == "jpg" || j.FileType == "jpeg" || j.FileType == "png"
}

func (j *PrintJob) IsOffice() bool {
	switch j.FileType {
	case "docx", "doc", "xlsx", "xls", "pptx", "ppt", "odt", "ods", "odp", "rtf":
		return true
	}
	return false
}

// Cache — потокобезопасный in-memory кэш заданий печати с TTL.
type Cache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]*PrintJob
}

func NewCache(ttlMinutes int) *Cache {
	c := &Cache{
		ttl: time.Duration(ttlMinutes) * time.Minute,
		m:   make(map[string]*PrintJob),
	}
	go c.cleanupLoop()
	return c
}

func key(chatID int64, messageID int) string {
	return fmt.Sprintf("%d:%d", chatID, messageID)
}

func (c *Cache) Set(job *PrintJob) {
	job.CreatedAt = time.Now()
	c.mu.Lock()
	c.m[key(job.ChatID, job.MessageID)] = job
	c.mu.Unlock()
}

func (c *Cache) Get(chatID int64, messageID int) (*PrintJob, bool) {
	c.mu.RLock()
	job, ok := c.m[key(chatID, messageID)]
	c.mu.RUnlock()
	return job, ok
}

func (c *Cache) Delete(chatID int64, messageID int) {
	c.mu.Lock()
	delete(c.m, key(chatID, messageID))
	c.mu.Unlock()
}

func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		for k, job := range c.m {
			if time.Since(job.CreatedAt) > c.ttl {
				delete(c.m, k)
			}
		}
		c.mu.Unlock()
	}
}
