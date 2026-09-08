// Package clock абстрагирует получение текущего времени, чтобы домен и
// application-слой оставались тестируемыми без реального time.Now().
package clock

import "time"

// Clock — источник текущего времени.
type Clock interface {
	Now() time.Time
}

// Real — реализация Clock поверх time.Now.
type Real struct{}

// Now возвращает текущее время.
func (Real) Now() time.Time {
	return time.Now()
}

var _ Clock = Real{}
