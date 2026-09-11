// Package keyedmutex даёт блокировку по произвольному строковому ключу —
// используется оркестратором, чтобы не отправить два одновременных ордера
// по одной и той же бумаге из разных воркеров.
package keyedmutex

import "sync"

// Mutex — набор независимых мьютексов, адресуемых строковым ключом.
// Карта локов никогда не уменьшается: для ограниченного watchlist это не
// проблема, но не подходит для неограниченного множества ключей.
type Mutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// New создаёт пустой Mutex.
func New() *Mutex {
	return &Mutex{locks: make(map[string]*sync.Mutex)}
}

// Lock блокирует key и возвращает функцию для разблокировки.
func (m *Mutex) Lock(key string) (unlock func()) {
	m.mu.Lock()
	l, ok := m.locks[key]
	if !ok {
		l = &sync.Mutex{}
		m.locks[key] = l
	}
	m.mu.Unlock()

	l.Lock()
	return l.Unlock
}
