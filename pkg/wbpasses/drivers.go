package wbpasses

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	driversFile = "drivers.txt"
	driversMu   sync.RWMutex
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

		// Убираем префикс "Driver: " если есть
		line = strings.TrimPrefix(line, "Driver: ")
		parts := strings.Split(line, ";")
		if len(parts) < 9 {
			continue
		}

		requiredPass, _ := strconv.Atoi(strings.TrimSpace(parts[5]))
		officeID, _ := strconv.Atoi(strings.TrimSpace(parts[6]))
		active, _ := strconv.ParseBool(strings.TrimSpace(parts[8]))

		driver := Driver{
			ID:           id,
			LastName:     strings.TrimSpace(parts[0]),
			FirstName:    strings.TrimSpace(parts[1]),
			CarModel:     strings.TrimSpace(parts[2]),
			CarNumber:    strings.TrimSpace(parts[3]),
			Phone:        strings.TrimSpace(parts[4]),
			RequiredPass: requiredPass,
			OfficeID:     officeID,
			OfficeName:   strings.TrimSpace(parts[7]),
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
	writer.WriteString("# Формат: Фамилия;Имя;Марка авто;Номер авто;Телефон;Количество нужных пропусков;ID склада;Название склада;Active (1/0)\n")
	writer.WriteString("# Active = 1 - проверять, Active = 0 - не проверять (очищенная строка)\n")

	for _, d := range drivers {
		active := "0"
		if d.Active {
			active = "1"
		}
		line := "Driver: " + d.LastName + ";" + d.FirstName + ";" + d.CarModel + ";" +
			d.CarNumber + ";" + d.Phone + ";" + strconv.Itoa(d.RequiredPass) + ";" +
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

// ToggleDriverActive - переключение активности водителя (очистка/восстановление)
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
