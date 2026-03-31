package wbpasses

import "time"

// Конфигурация
type Config struct {
	APIToken string
	APIURL   string
	Port     string
}

// Структура пропуска (ответ API)
type Pass struct {
	ID            int    `json:"id"`
	OfficeID      int64  `json:"officeId"`
	OfficeName    string `json:"officeName"`
	OfficeAddress string `json:"officeAddress"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	CarModel      string `json:"carModel"`
	CarNumber     string `json:"carNumber"`
	DateEnd       string `json:"dateEnd"`
}

// Водитель из файла
type Driver struct {
	ID           int    `json:"id"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	CarModel     string `json:"carModel"`
	CarNumber    string `json:"carNumber"`
	RequiredPass int    `json:"requiredPass"`
	OfficeID     int    `json:"officeId"`
	OfficeName   string `json:"officeName"`
	Active       bool   `json:"active"`
}

// Статус проверки пропусков для водителя
type DriverStatus struct {
	Driver      Driver `json:"driver"`
	Passes3Days int    `json:"passes3Days"`
	IsEnough    bool   `json:"isEnough"`
	StatusColor string `json:"statusColor"`
}

// Запрос на создание пропуска
type CreatePassRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	CarModel  string `json:"carModel"`
	CarNumber string `json:"carNumber"`
	OfficeID  int    `json:"officeId"`
}

// Ответ на создание пропуска
type CreatePassResponse struct {
	ID int `json:"id"`
}

// Склад
type Office struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Расширенная структура пропуска для отображения с цветом
type PassWithColor struct {
	Pass
	RowColor string `json:"rowColor"`
	DaysLeft int    `json:"daysLeft"`
}

// Задача на создание пропуска
type PassTask struct {
	DriverID  int       `json:"driverId"`
	Driver    Driver    `json:"driver"`
	CreatedAt time.Time `json:"createdAt"`
	Status    string    `json:"status"` // pending, processing, completed, failed
	Error     string    `json:"error,omitempty"`
}

// Очередь пропусков
type PassQueue struct {
	Tasks     []PassTask `json:"tasks"`
	LastRun   time.Time  `json:"lastRun"`
	IsRunning bool       `json:"isRunning"`
}
