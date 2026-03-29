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

// GetActivePassesWithColor - получение активных пропусков с цветовой индикацией
func GetActivePassesWithColor(apiToken, apiURL string) ([]PassWithColor, error) {
	passes, err := GetPasses(apiToken, apiURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var result []PassWithColor

	for _, p := range passes {
		endDate, err := time.Parse(time.RFC3339, p.DateEnd)
		if err != nil {
			continue
		}

		// Пропускаем просроченные
		if endDate.Before(now) {
			continue
		}

		daysLeft := int(endDate.Sub(now).Hours() / 24)

		rowColor := "yellow"
		if daysLeft >= 3 {
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

// CheckPassesAvailability - проверка актуальности пропусков (только ≥3 дней)
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

	// Фильтруем только активные с остатком >= 3 дней
	now := time.Now()
	var freshPasses []Pass
	for _, p := range passes {
		endDate, err := time.Parse(time.RFC3339, p.DateEnd)
		if err != nil {
			continue
		}
		daysLeft := int(endDate.Sub(now).Hours() / 24)
		if daysLeft >= 3 {
			freshPasses = append(freshPasses, p)
		}
	}

	// Для каждого водителя считаем пропуска
	var results []DriverStatus
	for _, driver := range drivers {
		status := DriverStatus{
			Driver:      driver,
			Passes3Days: 0,
			IsEnough:    false,
			StatusColor: "red",
		}

		// Считаем пропуска с остатком >= 3 дней
		for _, pass := range freshPasses {
			if pass.LastName == driver.LastName &&
				pass.FirstName == driver.FirstName &&
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
