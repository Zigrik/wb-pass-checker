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
	Phone        string `json:"phone"`
	RequiredPass int    `json:"requiredPass"` // Количество нужных пропусков (срок ≥3 дней)
	OfficeID     int    `json:"officeId"`
	OfficeName   string `json:"officeName"`
}

// Статус проверки пропусков для водителя
type DriverStatus struct {
	Driver      Driver `json:"driver"`
	Passes3Days int    `json:"passes3Days"` // Пропусков с остатком ≥3 дней
	IsEnough    bool   `json:"isEnough"`    // Достаточно ли пропусков
	StatusColor string `json:"statusColor"` // green или red
}

// Запрос на создание пропуска
type CreatePassRequest struct {
	OrderID     int64  `json:"orderId"`
	PassType    string `json:"passType"`
	VehicleInfo string `json:"vehicleInfo"`
	DriverName  string `json:"driverName"`
	DriverPhone string `json:"driverPhone"`
	OfficeID    int    `json:"officeId"`
}

// Ответ на создание пропуска
type CreatePassResponse struct {
	ID        int       `json:"id"`
	OrderID   int64     `json:"orderId"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	QRCode    string    `json:"qrCode"`
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
	RowColor string `json:"rowColor"` // green, yellow
	DaysLeft int    `json:"daysLeft"`
}
