package wbpasses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var client = &http.Client{Timeout: 30 * time.Second}

// GetPasses - получение списка пропусков
func GetPasses(apiToken, apiURL string) ([]Pass, error) {
	url := fmt.Sprintf("%s/api/v3/passes", apiURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var passes []Pass
	if err := json.Unmarshal(body, &passes); err != nil {
		return nil, err
	}

	return passes, nil
}

// CreatePass - создание пропуска
func CreatePass(apiToken, apiURL string, req CreatePassRequest) (*CreatePassResponse, error) {
	requestBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v3/passes", apiURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", apiToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var createResponse CreatePassResponse
	if err := json.Unmarshal(body, &createResponse); err != nil {
		return nil, err
	}

	return &createResponse, nil
}

// GetOffices - получение складов
func GetOffices(apiToken, apiURL string) ([]Office, error) {
	url := fmt.Sprintf("%s/api/v3/offices", apiURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var offices []Office
	if err := json.Unmarshal(body, &offices); err != nil {
		return nil, err
	}

	return offices, nil
}

// normalizeTo3AM - нормализует время до 03:00 для корректного сравнения
func normalizeTo3AM(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 3, 0, 0, 0, t.Location())
}

// GetFreshDate - возвращает дату, до которой считаются свежие пропуска
// Свежий пропуск: дата окончания >= сегодня + 2 дня (с учетом времени 03:00)
func GetFreshDate(now time.Time) time.Time {
	// Получаем сегодняшнюю дату в 00:00
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// Добавляем 2 дня
	freshDate := today.AddDate(0, 0, 2)
	// Устанавливаем время 03:00
	return time.Date(freshDate.Year(), freshDate.Month(), freshDate.Day(), 3, 0, 0, 0, freshDate.Location())
}

// GetActivePassesWithColor - получение активных пропусков с цветовой индикацией
func GetActivePassesWithColor(apiToken, apiURL string) ([]PassWithColor, error) {
	passes, err := GetPasses(apiToken, apiURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	freshDate := GetFreshDate(now)

	var result []PassWithColor

	for _, p := range passes {
		endDate, err := time.Parse(time.RFC3339, p.DateEnd)
		if err != nil {
			continue
		}

		// Пропускаем просроченные (дата окончания < сегодня 00:00)
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if endDate.Before(todayStart) {
			continue
		}

		// Вычисляем остаток дней с учетом времени 03:00
		// Нормализуем endDate до 03:00 для корректного сравнения
		endDateNormalized := normalizeTo3AM(endDate)
		nowNormalized := normalizeTo3AM(now)

		daysLeft := int(endDateNormalized.Sub(nowNormalized).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}

		// Определяем цвет строки:
		// Зеленый - свежий пропуск (дата окончания >= freshDate)
		// Желтый - активный, но не свежий
		rowColor := "yellow"
		if !endDate.Before(freshDate) {
			rowColor = "green"
		}

		result = append(result, PassWithColor{
			Pass:     p,
			RowColor: rowColor,
			DaysLeft: daysLeft,
		})
	}

	// Сортировка от новых к старым
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			dateI, _ := time.Parse(time.RFC3339, result[i].DateEnd)
			dateJ, _ := time.Parse(time.RFC3339, result[j].DateEnd)
			if dateI.Before(dateJ) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// CheckPassesAvailability - проверка актуальности пропусков
// Свежие = дата окончания >= сегодня+2 дня (с учетом времени 03:00)
func CheckPassesAvailability(apiToken, apiURL string) ([]DriverStatus, error) {
	// Загружаем водителей
	drivers, err := LoadDrivers()
	if err != nil {
		return nil, err
	}

	// Получаем все пропуска
	passes, err := GetPasses(apiToken, apiURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	freshDate := GetFreshDate(now)

	// Фильтруем только свежие пропуска (дата окончания >= freshDate)
	var freshPasses []Pass
	for _, p := range passes {
		endDate, err := time.Parse(time.RFC3339, p.DateEnd)
		if err != nil {
			continue
		}
		// Пропуск свежий, если дата окончания >= freshDate
		if !endDate.Before(freshDate) {
			freshPasses = append(freshPasses, p)
		}
	}

	// Для каждого водителя считаем свежие пропуска
	var results []DriverStatus
	for _, driver := range drivers {
		status := DriverStatus{
			Driver:      driver,
			Passes3Days: 0,
			IsEnough:    false,
			StatusColor: "red",
		}

		// Если водитель неактивен (очищен), показываем серым
		if !driver.Active {
			status.StatusColor = "grey"
			results = append(results, status)
			continue
		}

		// Считаем свежие пропуска для этого водителя
		for _, pass := range freshPasses {
			if pass.LastName == driver.LastName &&
				pass.FirstName == driver.FirstName &&
				pass.CarModel == driver.CarModel &&
				pass.CarNumber == driver.CarNumber &&
				pass.OfficeID == int64(driver.OfficeID) {
				status.Passes3Days++
			}
		}

		if status.Passes3Days >= driver.RequiredPass {
			status.IsEnough = true
			status.StatusColor = "green"
		}

		results = append(results, status)
	}

	return results, nil
}

// CreatePassesForDriver - создание нескольких пропусков для водителя
func CreatePassesForDriver(apiToken, apiURL string, driver Driver, count int) ([]CreatePassResponse, error) {
	var results []CreatePassResponse

	for i := 0; i < count; i++ {
		// Формируем запрос на создание пропуска
		req := CreatePassRequest{
			OrderID:     int64(time.Now().UnixNano() + int64(i)), // Уникальный ID заказа
			PassType:    "entry",
			VehicleInfo: driver.CarModel + " " + driver.CarNumber,
			DriverName:  driver.LastName + " " + driver.FirstName,
			DriverPhone: driver.Phone,
			OfficeID:    driver.OfficeID,
		}

		resp, err := CreatePass(apiToken, apiURL, req)
		if err != nil {
			return results, fmt.Errorf("failed to create pass %d for driver %s: %w", i+1, driver.LastName, err)
		}
		results = append(results, *resp)
	}

	return results, nil
}
