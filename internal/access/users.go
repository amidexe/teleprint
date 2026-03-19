package access

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Manager struct {
	mu        sync.RWMutex
	adminID   int64
	users     map[int64]bool
	usersFile string
}

func NewManager(adminID int64, dataDir string) *Manager {
	usersFile := "users.json"
	if dataDir != "" {
		usersFile = filepath.Join(dataDir, "users.json")
	}
	m := &Manager{
		adminID:   adminID,
		users:     make(map[int64]bool),
		usersFile: usersFile,
	}
	m.load()
	return m
}

// IsAllowed проверяет есть ли у пользователя доступ.
func (m *Manager) IsAllowed(userID int64) bool {
	if userID == m.adminID {
		return true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.users[userID]
}

// IsAdmin проверяет является ли пользователь супер-админом.
func (m *Manager) IsAdmin(userID int64) bool {
	return userID == m.adminID
}

// Add добавляет пользователя. Возвращает false если уже был добавлен.
func (m *Manager) Add(userID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.users[userID] {
		return false
	}
	m.users[userID] = true
	m.save()
	return true
}

// Remove удаляет пользователя. Возвращает false если не был найден.
func (m *Manager) Remove(userID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.users[userID] {
		return false
	}
	delete(m.users, userID)
	m.save()
	return true
}

// List возвращает список всех разрешённых пользователей (без админа).
func (m *Manager) List() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]int64, 0, len(m.users))
	for id := range m.users {
		ids = append(ids, id)
	}
	return ids
}

// load читает users.json. Ошибка игнорируется — файл создастся при первом сохранении.
func (m *Manager) load() {
	data, err := os.ReadFile(m.usersFile)
	if err != nil {
		return
	}
	var ids []int64
	if err := json.Unmarshal(data, &ids); err != nil {
		return
	}
	for _, id := range ids {
		m.users[id] = true
	}
}

// save атомично записывает users.json через временный файл.
func (m *Manager) save() {
	ids := make([]int64, 0, len(m.users))
	for id := range m.users {
		ids = append(ids, id)
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return
	}
	tmp := m.usersFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tmp, m.usersFile)
}
