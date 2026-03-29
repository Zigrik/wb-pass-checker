package wbpasses

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Server - структура веб-сервера
type Server struct {
	config Config
	mux    *http.ServeMux
}

// NewServer - создание нового сервера
func NewServer(config Config) *Server {
	s := &Server{
		config: config,
		mux:    http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

// setupRoutes - настройка маршрутов
func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/passes", s.handleGetPasses)
	s.mux.HandleFunc("/api/passes/create", s.handleCreatePass)
	s.mux.HandleFunc("/api/offices", s.handleGetOffices)
	s.mux.HandleFunc("/api/drivers", s.handleDrivers)
	s.mux.HandleFunc("/api/drivers/status", s.handleDriversStatus)
	s.mux.HandleFunc("/api/check-availability", s.handleCheckAvailability)
}

// Start - запуск сервера
func (s *Server) Start() error {
	server := &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Printf("🚀 Server starting on http://localhost:%s", s.config.Port)
	log.Printf("📋 API URL: %s", s.config.APIURL)
	log.Printf("🔑 API Token: %s...", s.config.APIToken[:15])
	return server.ListenAndServe()
}

// GetDriverStatus - публичная функция для проверки статуса водителей (для импорта)
func GetDriverStatus(apiToken, apiURL string) ([]DriverStatus, error) {
	return CheckPassesAvailability(apiToken, apiURL)
}

// GetActivePasses - публичная функция для получения активных пропусков
func GetActivePasses(apiToken, apiURL string) ([]PassWithColor, error) {
	return GetActivePassesWithColor(apiToken, apiURL)
}

// handleIndex - главная страница
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

// handleGetPasses - получение списка пропусков
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

// handleCreatePass - создание пропуска
func (s *Server) handleCreatePass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreatePassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.OrderID == 0 {
		http.Error(w, "OrderID is required", http.StatusBadRequest)
		return
	}
	if req.VehicleInfo == "" {
		http.Error(w, "VehicleInfo is required", http.StatusBadRequest)
		return
	}
	if req.DriverName == "" {
		http.Error(w, "DriverName is required", http.StatusBadRequest)
		return
	}
	if req.DriverPhone == "" {
		http.Error(w, "DriverPhone is required", http.StatusBadRequest)
		return
	}
	if req.OfficeID == 0 {
		http.Error(w, "OfficeID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Creating pass for order %d at office %d", req.OrderID, req.OfficeID)

	response, err := CreatePass(s.config.APIToken, s.config.APIURL, req)
	if err != nil {
		log.Printf("Error creating pass: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleGetOffices - получение складов
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

// handleDrivers - CRUD для водителей
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

// handleDriversStatus - получение статуса водителей
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

// handleCheckAvailability - проверка доступности пропусков (для внешнего использования)
func (s *Server) handleCheckAvailability(w http.ResponseWriter, r *http.Request) {
	statuses, err := CheckPassesAvailability(s.config.APIToken, s.config.APIURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allEnough := true
	for _, st := range statuses {
		if !st.IsEnough {
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
