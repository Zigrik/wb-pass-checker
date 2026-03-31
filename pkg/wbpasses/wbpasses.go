package wbpasses

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	config Config
	mux    *http.ServeMux
	stopCh chan struct{}
}

func NewServer(config Config) *Server {
	s := &Server{
		config: config,
		mux:    http.NewServeMux(),
		stopCh: make(chan struct{}),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/passes", s.handleGetPasses)
	s.mux.HandleFunc("/api/passes/create", s.handleCreatePass)
	s.mux.HandleFunc("/api/passes/create-batch", s.handleCreatePassesBatch)
	s.mux.HandleFunc("/api/passes/create-all", s.handleCreatePassesForAll)
	s.mux.HandleFunc("/api/passes/queue", s.handleGetQueue)
	s.mux.HandleFunc("/api/passes/queue/clear", s.handleClearQueue)
	s.mux.HandleFunc("/api/offices", s.handleGetOffices)
	s.mux.HandleFunc("/api/drivers", s.handleDrivers)
	s.mux.HandleFunc("/api/drivers/status", s.handleDriversStatus)
	s.mux.HandleFunc("/api/drivers/toggle", s.handleToggleDriverActive)
	s.mux.HandleFunc("/api/check-availability", s.handleCheckAvailability)
}

func (s *Server) Start() error {
	go s.scheduler()

	server := &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Printf("🚀 Server starting on http://localhost:%s", s.config.Port)
	log.Printf("📋 API URL: %s", s.config.APIURL)
	log.Printf("🔑 API Token: %s...", s.config.APIToken[:15])
	log.Printf("⏰ Планировщик запущен: обновление каждые 3 минуты, создание пропусков по очереди (1 раз в 10 минут)")
	return server.ListenAndServe()
}

func (s *Server) Stop() {
	close(s.stopCh)
}

func (s *Server) scheduler() {
	s.processQueue()

	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("🔄 Планировщик: обновление данных и проверка очереди")
			s.processQueue()
		case <-s.stopCh:
			log.Println("⏹️ Планировщик остановлен")
			return
		}
	}
}

func (s *Server) processQueue() {
	size, err := GetQueueSize()
	if err != nil {
		log.Printf("❌ Ошибка получения размера очереди: %v", err)
		return
	}

	if size == 0 {
		log.Println("📭 Очередь пуста")
		return
	}

	log.Printf("📋 В очереди %d задач", size)

	processed, err := ProcessNextTask(s.config.APIToken, s.config.APIURL)
	if err != nil {
		log.Printf("❌ Ошибка обработки задачи: %v", err)
		return
	}

	if processed {
		log.Println("✅ Обработана одна задача из очереди")
	} else {
		queue, _ := LoadQueue()
		if queue.IsRunning {
			log.Println("⏳ Обработка уже выполняется")
		} else if !queue.LastRun.IsZero() && time.Since(queue.LastRun) < 630*time.Second {
			waitTime := 630*time.Second - time.Since(queue.LastRun)
			log.Printf("⏰ Ожидание %v до следующего создания пропуска", waitTime.Round(time.Second))
		}
	}
}

func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	queue, err := LoadQueue()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pendingCount := 0
	for _, task := range queue.Tasks {
		if task.Status == "pending" {
			pendingCount++
		}
	}

	// Интервал 10 минут 30 секунд (630 секунд)
	nextRunTime := queue.LastRun.Add(630 * time.Second)
	canRun := time.Now().After(nextRunTime) && !queue.IsRunning

	// Вычисляем время до следующего запуска в секундах
	var secondsUntilNext int64 = 0
	if !canRun && !queue.LastRun.IsZero() {
		remaining := nextRunTime.Sub(time.Now())
		if remaining > 0 {
			secondsUntilNext = int64(remaining.Seconds())
		}
	}

	result := map[string]interface{}{
		"pending":          pendingCount,
		"total":            len(queue.Tasks),
		"isRunning":        queue.IsRunning,
		"lastRun":          queue.LastRun,
		"nextRunTime":      nextRunTime,
		"canRun":           canRun,
		"secondsUntilNext": secondsUntilNext,
		"tasks":            queue.Tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

func (s *Server) handleGetPasses(w http.ResponseWriter, r *http.Request) {
	passes, err := GetActivePassesWithColor(s.config.APIToken, s.config.APIURL)
	if err != nil {
		log.Printf("Error getting passes: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(passes)
}

func (s *Server) handleCreatePass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DriverID int `json:"driverId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	drivers, err := LoadDrivers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var targetDriver *Driver
	for i, d := range drivers {
		if d.ID == req.DriverID {
			targetDriver = &drivers[i]
			break
		}
	}

	if targetDriver == nil {
		http.Error(w, "Driver not found", http.StatusNotFound)
		return
	}

	// Добавляем в очередь
	if err := AddToQueue(*targetDriver); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Запускаем обработку очереди
	go s.processQueue()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Пропуск добавлен в очередь. Будет создан автоматически (1 раз в 10 минут)",
	})
}

func (s *Server) handleCreatePassesBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DriverID int `json:"driverId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	drivers, err := LoadDrivers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var targetDriver *Driver
	for i, d := range drivers {
		if d.ID == req.DriverID {
			targetDriver = &drivers[i]
			break
		}
	}

	if targetDriver == nil {
		http.Error(w, "Driver not found", http.StatusNotFound)
		return
	}

	// Добавляем в очередь (1 пропуск)
	if err := AddToQueue(*targetDriver); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go s.processQueue()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Пропуск добавлен в очередь. Будет создан автоматически (1 раз в 10 минут)",
	})
}

func (s *Server) handleCreatePassesForAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("📦 Добавление всех недостающих пропусков в очередь")

	statuses, err := CheckPassesAvailability(s.config.APIToken, s.config.APIURL)
	if err != nil {
		log.Printf("❌ Ошибка получения статусов: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var driversToAdd []Driver
	for _, status := range statuses {
		if status.Driver.Active && !status.IsEnough {
			needed := status.Driver.RequiredPass - status.Passes3Days
			if needed > 0 {
				log.Printf("  Водитель %s %s: нужно %d пропусков",
					status.Driver.LastName, status.Driver.FirstName, needed)
				for i := 0; i < needed; i++ {
					driversToAdd = append(driversToAdd, status.Driver)
				}
			}
		}
	}

	if len(driversToAdd) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Нет водителей, нуждающихся в пропусках",
			"added":   0,
		})
		return
	}

	if err := AddMultipleToQueue(driversToAdd); err != nil {
		log.Printf("❌ Ошибка добавления в очередь: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Добавлено %d заданий в очередь", len(driversToAdd))

	go s.processQueue()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Добавлено %d заданий в очередь. Пропуска будут создаваться автоматически (1 раз в 10 минут)", len(driversToAdd)),
		"added":   len(driversToAdd),
	})
}

func (s *Server) handleClearQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queue, err := LoadQueue()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	queue.Tasks = []PassTask{}
	queue.IsRunning = false

	if err := SaveQueue(queue); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Очередь очищена",
	})
}

func (s *Server) handleGetOffices(w http.ResponseWriter, r *http.Request) {
	offices, err := GetOffices(s.config.APIToken, s.config.APIURL)
	if err != nil {
		log.Printf("Error getting offices: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(offices)
}

func (s *Server) handleDrivers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		drivers, err := LoadDrivers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(drivers)

	case http.MethodPost:
		var driver Driver
		if err := json.NewDecoder(r.Body).Decode(&driver); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := AddDriver(driver); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(driver)

	case http.MethodPut:
		var driver Driver
		if err := json.NewDecoder(r.Body).Decode(&driver); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := UpdateDriver(driver.ID, driver); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(driver)

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}
		if err := DeleteDriver(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleToggleDriverActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	if err := ToggleDriverActive(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleDriversStatus(w http.ResponseWriter, r *http.Request) {
	statuses, err := CheckPassesAvailability(s.config.APIToken, s.config.APIURL)
	if err != nil {
		log.Printf("Error checking drivers status: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}

func (s *Server) handleCheckAvailability(w http.ResponseWriter, r *http.Request) {
	statuses, err := CheckPassesAvailability(s.config.APIToken, s.config.APIURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allEnough := true
	for _, st := range statuses {
		if st.Driver.Active && !st.IsEnough {
			allEnough = false
			break
		}
	}

	result := map[string]interface{}{
		"all_enough": allEnough,
		"details":    statuses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
