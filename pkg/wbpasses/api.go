package wbpasses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	requestBody := map[string]interface{}{
		"firstName": req.FirstName,
		"lastName":  req.LastName,
		"carModel":  req.CarModel,
		"carNumber": req.CarNumber,
		"officeId":  req.OfficeID,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("=== СОЗДАНИЕ ПРОПУСКА ===")
	log.Printf("URL: %s/api/v3/passes", apiURL)
	log.Printf("Body: %s", string(jsonBody))

	url := fmt.Sprintf("%s/api/v3/passes", apiURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	log.Printf("Response status: %d, body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var createResponse CreatePassResponse
	if err := json.Unmarshal(body, &createResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("✅ Пропуск успешно создан, ID: %d", createResponse.ID)
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

// GetFreshDate - возвращает дату, до которой считаются свежие пропуска
func GetFreshDate(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	freshDate := today.AddDate(0, 0, 2)
	return freshDate
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

		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if endDate.Before(todayStart) {
			continue
		}

		daysLeft := int(endDate.Sub(now).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}

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
func CheckPassesAvailability(apiToken, apiURL string) ([]DriverStatus, error) {
	drivers, err := LoadDrivers()
	if err != nil {
		return nil, err
	}

	passes, err := GetPasses(apiToken, apiURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	freshDate := GetFreshDate(now)

	var freshPasses []Pass
	for _, p := range passes {
		endDate, err := time.Parse(time.RFC3339, p.DateEnd)
		if err != nil {
			continue
		}
		if p.OfficeID == 301983 && !endDate.Before(freshDate) {
			freshPasses = append(freshPasses, p)
		}
	}

	log.Printf("=== СВЕЖИЕ ПРОПУСКА (склад 301983, дата >= %s) ===", freshDate.Format("2006-01-02"))
	for _, p := range freshPasses {
		log.Printf("  ID=%d: %s %s, авто: %s %s, склад: %d, дата: %s",
			p.ID, p.LastName, p.FirstName, p.CarModel, p.CarNumber, p.OfficeID, p.DateEnd)
	}

	var results []DriverStatus
	for _, driver := range drivers {
		status := DriverStatus{
			Driver:      driver,
			Passes3Days: 0,
			IsEnough:    false,
			StatusColor: "red",
		}

		if !driver.Active {
			status.StatusColor = "grey"
			results = append(results, status)
			continue
		}

		for _, pass := range freshPasses {
			if pass.LastName == driver.LastName &&
				pass.FirstName == driver.FirstName &&
				pass.CarModel == driver.CarModel &&
				pass.CarNumber == driver.CarNumber {
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

// CreatePassesForDriver - создание нескольких пропусков для водителя (для одиночного заказа)
func CreatePassesForDriver(apiToken, apiURL string, driver Driver, count int) ([]CreatePassResponse, error) {
	var results []CreatePassResponse

	log.Printf("Создание %d пропусков для водителя %s %s", count, driver.LastName, driver.FirstName)

	for i := 0; i < count; i++ {
		req := CreatePassRequest{
			FirstName: driver.FirstName,
			LastName:  driver.LastName,
			CarModel:  driver.CarModel,
			CarNumber: driver.CarNumber,
			OfficeID:  driver.OfficeID,
		}

		log.Printf("  Создание пропуска %d: водитель %s %s, авто %s %s, офис %d",
			i+1, req.LastName, req.FirstName, req.CarModel, req.CarNumber, req.OfficeID)

		resp, err := CreatePass(apiToken, apiURL, req)
		if err != nil {
			return results, fmt.Errorf("failed to create pass %d for driver %s: %w", i+1, driver.LastName, err)
		}
		results = append(results, *resp)

		if i < count-1 {
			time.Sleep(250 * time.Millisecond)
		}
	}

	log.Printf("Успешно создано %d пропусков для водителя %s %s", len(results), driver.LastName, driver.FirstName)
	return results, nil
}
