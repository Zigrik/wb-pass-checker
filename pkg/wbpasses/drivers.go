package wbpasses

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	driversFile = "drivers.txt"
	driversMu   sync.RWMutex
	queueFile   = "queue.json"
	queueMu     sync.RWMutex
)

// LoadDrivers - загрузка водителей из файла
func LoadDrivers() ([]Driver, error) {
	driversMu.RLock()
	defer driversMu.RUnlock()

	file, err := os.Open(driversFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Driver{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var drivers []Driver
	scanner := bufio.NewScanner(file)
	id := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "Driver: ")
		parts := strings.Split(line, ";")
		if len(parts) < 8 {
			continue
		}

		requiredPass, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
		officeID, _ := strconv.Atoi(strings.TrimSpace(parts[5]))
		active, _ := strconv.ParseBool(strings.TrimSpace(parts[7]))

		driver := Driver{
			ID:           id,
			LastName:     strings.TrimSpace(parts[0]),
			FirstName:    strings.TrimSpace(parts[1]),
			CarModel:     strings.TrimSpace(parts[2]),
			CarNumber:    strings.TrimSpace(parts[3]),
			RequiredPass: requiredPass,
			OfficeID:     officeID,
			OfficeName:   strings.TrimSpace(parts[6]),
			Active:       active,
		}
		drivers = append(drivers, driver)
		id++
	}

	return drivers, scanner.Err()
}

// SaveDrivers - сохранение водителей в файл
func SaveDrivers(drivers []Driver) error {
	driversMu.Lock()
	defer driversMu.Unlock()

	file, err := os.Create(driversFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	writer.WriteString("# Формат: Фамилия;Имя;Марка авто;Номер авто;Количество нужных пропусков;ID склада;Название склада;Active (1/0)\n")
	writer.WriteString("# Active = 1 - проверять, Active = 0 - не проверять (очищенная строка)\n")

	for _, d := range drivers {
		active := "0"
		if d.Active {
			active = "1"
		}
		line := "Driver: " + d.LastName + ";" + d.FirstName + ";" + d.CarModel + ";" +
			d.CarNumber + ";" + strconv.Itoa(d.RequiredPass) + ";" +
			strconv.Itoa(d.OfficeID) + ";" + d.OfficeName + ";" + active + "\n"
		writer.WriteString(line)
	}
	return writer.Flush()
}

// AddDriver - добавление нового водителя
func AddDriver(driver Driver) error {
	drivers, err := LoadDrivers()
	if err != nil {
		return err
	}

	driver.OfficeID = 301983
	driver.OfficeName = "Волгоград"

	driver.ID = len(drivers) + 1
	drivers = append(drivers, driver)
	return SaveDrivers(drivers)
}

// UpdateDriver - обновление водителя
func UpdateDriver(id int, driver Driver) error {
	drivers, err := LoadDrivers()
	if err != nil {
		return err
	}

	driver.OfficeID = 301983
	driver.OfficeName = "Волгоград"

	for i, d := range drivers {
		if d.ID == id {
			driver.ID = id
			drivers[i] = driver
			break
		}
	}
	return SaveDrivers(drivers)
}

// DeleteDriver - удаление водителя
func DeleteDriver(id int) error {
	drivers, err := LoadDrivers()
	if err != nil {
		return err
	}

	newDrivers := make([]Driver, 0, len(drivers)-1)
	for _, d := range drivers {
		if d.ID != id {
			newDrivers = append(newDrivers, d)
		}
	}
	return SaveDrivers(newDrivers)
}

// ToggleDriverActive - переключение активности водителя
func ToggleDriverActive(id int) error {
	drivers, err := LoadDrivers()
	if err != nil {
		return err
	}

	for i, d := range drivers {
		if d.ID == id {
			drivers[i].Active = !drivers[i].Active
			break
		}
	}
	return SaveDrivers(drivers)
}

// LoadQueue - загрузка очереди из файла
func LoadQueue() (*PassQueue, error) {
	queueMu.RLock()
	defer queueMu.RUnlock()

	file, err := os.Open(queueFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &PassQueue{
				Tasks:     []PassTask{},
				LastRun:   time.Time{},
				IsRunning: false,
			}, nil
		}
		return nil, err
	}
	defer file.Close()

	var queue PassQueue
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// SaveQueue - сохранение очереди в файл
func SaveQueue(queue *PassQueue) error {
	queueMu.Lock()
	defer queueMu.Unlock()

	file, err := os.Create(queueFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(queue)
}

// AddToQueue - добавление задачи в очередь
func AddToQueue(driver Driver) error {
	queue, err := LoadQueue()
	if err != nil {
		return err
	}

	for _, task := range queue.Tasks {
		if task.DriverID == driver.ID && task.Status == "pending" {
			return nil
		}
	}

	task := PassTask{
		DriverID:  driver.ID,
		Driver:    driver,
		CreatedAt: time.Now(),
		Status:    "pending",
	}

	queue.Tasks = append(queue.Tasks, task)
	return SaveQueue(queue)
}

// AddMultipleToQueue - добавление нескольких задач в очередь (очищая старую)
func AddMultipleToQueue(drivers []Driver) error {
	queue, err := LoadQueue()
	if err != nil {
		return err
	}

	queue.Tasks = []PassTask{}
	queue.IsRunning = false

	for _, driver := range drivers {
		if driver.Active {
			task := PassTask{
				DriverID:  driver.ID,
				Driver:    driver,
				CreatedAt: time.Now(),
				Status:    "pending",
			}
			queue.Tasks = append(queue.Tasks, task)
		}
	}

	return SaveQueue(queue)
}

// GetQueueSize - получение размера очереди
func GetQueueSize() (int, error) {
	queue, err := LoadQueue()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, task := range queue.Tasks {
		if task.Status == "pending" {
			count++
		}
	}
	return count, nil
}

// ProcessNextTask - обработка следующей задачи из очереди
func ProcessNextTask(apiToken, apiURL string) (bool, error) {
	queue, err := LoadQueue()
	if err != nil {
		return false, err
	}

	var taskIndex int = -1
	for i, task := range queue.Tasks {
		if task.Status == "pending" {
			taskIndex = i
			break
		}
	}

	if taskIndex == -1 {
		return false, nil
	}

	if queue.IsRunning {
		return false, nil
	}

	// Изменяем интервал на 10 минут 30 секунд (630 секунд)
	if !queue.LastRun.IsZero() && time.Since(queue.LastRun) < 630*time.Second {
		return false, nil
	}

	queue.IsRunning = true
	queue.Tasks[taskIndex].Status = "processing"
	if err := SaveQueue(queue); err != nil {
		return false, err
	}

	task := queue.Tasks[taskIndex]

	req := CreatePassRequest{
		FirstName: task.Driver.FirstName,
		LastName:  task.Driver.LastName,
		CarModel:  task.Driver.CarModel,
		CarNumber: task.Driver.CarNumber,
		OfficeID:  task.Driver.OfficeID,
	}

	resp, err := CreatePass(apiToken, apiURL, req)

	queue, _ = LoadQueue()
	for i := range queue.Tasks {
		if queue.Tasks[i].DriverID == task.DriverID && queue.Tasks[i].Status == "processing" {
			if err != nil {
				queue.Tasks[i].Status = "failed"
				queue.Tasks[i].Error = err.Error()
			} else {
				queue.Tasks[i].Status = "completed"
				queue.Tasks[i].Error = ""
			}
			break
		}
	}

	queue.LastRun = time.Now()
	queue.IsRunning = false

	var newTasks []PassTask
	for _, t := range queue.Tasks {
		if t.Status == "pending" {
			newTasks = append(newTasks, t)
		} else if t.Status == "failed" && time.Since(t.CreatedAt) < 1*time.Hour {
			newTasks = append(newTasks, t)
		}
	}
	queue.Tasks = newTasks

	if err := SaveQueue(queue); err != nil {
		return false, err
	}

	if resp != nil {
		log.Printf("✅ Пропуск создан: ID=%d", resp.ID)
	}

	return true, nil
}
