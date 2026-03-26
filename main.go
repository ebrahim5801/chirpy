package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"ebrahim5801/chirpy/internal/auth"
	"ebrahim5801/chirpy/internal/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	jwtToken       string
	polkaKey       string
}

type errorResponse struct {
	Error string `json:"error"`
}

type User struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

type userRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ChirpResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type ChirpRequest struct {
	Body string `json:"body"`
}

type loginResponse struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type webhooksRequest struct {
	Event string      `json:"event"`
	Data  webhookData `json:"data"`
}

type webhookData struct {
	UserID uuid.UUID `json:"user_id"`
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	jwtToken := os.Getenv("JWT_TOKEN")
	polkaKey := os.Getenv("POLKA_KEY")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("connection error: %s", err)
	}
	dbQueries := database.New(db)

	apiCfg := &apiConfig{dbQueries: dbQueries, jwtToken: jwtToken, polkaKey: polkaKey}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", appHandler)

	fileServer := http.StripPrefix(
		"/app",
		http.FileServer(http.Dir(".")),
	)

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(fileServer))
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("POST /api/login", apiCfg.loginHandler)
	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("PUT /api/users", apiCfg.updateUserHandler)
	mux.HandleFunc("POST /api/refresh", apiCfg.refreshTokenHandler)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeTokenHandler)
	mux.HandleFunc("POST /api/chirps", apiCfg.createChirpHandler)
	mux.HandleFunc("GET /api/chirps", apiCfg.getChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirpHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirpHandler)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.polkaWebhooksHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	server.ListenAndServe()
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(data)
}

func respondWithError(w http.ResponseWriter, statusCode int, msg string) {
	respondWithJSON(w, statusCode, errorResponse{Error: msg})
}

func appHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
		</html>`, cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request) {
	if os.Getenv("PLATFORM") != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	cfg.dbQueries.DeleteUsers(req.Context())
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userID, err := auth.ValidateJWT(tokenString, cfg.jwtToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	data := ChirpRequest{}
	err = json.NewDecoder(req.Body).Decode(&data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	if len(data.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	data.Body = replaceProfaneWords(data.Body)

	chirpParam := database.CreateChirpParams{
		Body:   data.Body,
		UserID: userID,
	}

	chirpData, err := cfg.dbQueries.CreateChirp(req.Context(), chirpParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating a chirp")
		return
	}

	respondWithJSON(w, http.StatusCreated, ChirpResponse{
		ID:        chirpData.ID,
		CreatedAt: chirpData.CreatedAt,
		UpdatedAt: chirpData.UpdatedAt,
		Body:      chirpData.Body,
		UserID:    chirpData.UserID,
	})
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, req *http.Request) {
	chirps, err := cfg.dbQueries.GetChirps(req.Context())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error fetching data")
		return
	}

	chirpResponses := make([]ChirpResponse, len(chirps))
	for i, c := range chirps {
		chirpResponses[i] = ChirpResponse{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		}
	}

	respondWithJSON(w, http.StatusOK, chirpResponses)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, req *http.Request) {
	chirpID, err := uuid.Parse(req.PathValue("chirpID"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(req.Context(), chirpID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	respondWithJSON(w, http.StatusOK, ChirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func replaceProfaneWords(text string) string {
	profaneWords := map[string]bool{
		"kerfuffle": true,
		"sharbert":  true,
		"fornax":    true,
	}

	words := strings.Split(text, " ")

	for i, word := range words {
		if profaneWords[strings.ToLower(word)] {
			words[i] = "****"
		}
	}

	return strings.Join(words, " ")
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	data := userRequest{}
	err := json.NewDecoder(req.Body).Decode(&data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	hashedPassword, err := auth.HashPassword(data.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	UserParam := database.CreateUserParams{
		Email:    data.Email,
		Password: hashedPassword,
	}

	userData, err := cfg.dbQueries.CreateUser(req.Context(), UserParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating a user")
		return
	}

	respondWithJSON(w, http.StatusCreated, User{
		ID:          userData.ID,
		Email:       userData.Email,
		CreatedAt:   userData.CreatedAt,
		UpdatedAt:   userData.UpdatedAt,
		IsChirpyRed: userData.IsChirpyRed.Bool,
	})
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	data := userRequest{}
	err := json.NewDecoder(req.Body).Decode(&data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	user, err := cfg.dbQueries.GetUser(req.Context(), data.Email)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	allowLogin, err := auth.CheckPasswordHash(data.Password, user.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	if !allowLogin {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	expiry := time.Hour

	token, err := auth.MakeJWT(user.ID, cfg.jwtToken, expiry)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	refreshToken := auth.MakeRefreshToken()

	refreshTokenParams := database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(24 * 60 * time.Hour),
	}

	_, err = cfg.dbQueries.CreateRefreshToken(req.Context(), refreshTokenParams)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating refresh token")
		return
	}

	respondWithJSON(w, http.StatusOK, loginResponse{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
		IsChirpyRed:  user.IsChirpyRed.Bool,
	})
}

func (cfg *apiConfig) refreshTokenHandler(w http.ResponseWriter, req *http.Request) {
	requestRefreshToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	refreshToken, err := cfg.dbQueries.GetRefreshToken(req.Context(), requestRefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if time.Now().After(refreshToken.ExpiresAt) || refreshToken.RevokedAt.Valid {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	token, err := auth.MakeJWT(refreshToken.UserID, cfg.jwtToken, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	respondWithJSON(w, http.StatusOK, tokenResponse{
		Token: token,
	})
}

func (cfg *apiConfig) revokeTokenHandler(w http.ResponseWriter, req *http.Request) {
	requestRefreshToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	err = cfg.dbQueries.RevokeRefreshToken(req.Context(), requestRefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) updateUserHandler(w http.ResponseWriter, req *http.Request) {
	authToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	defer req.Body.Close()

	data := userRequest{}
	err = json.NewDecoder(req.Body).Decode(&data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	userID, err := auth.ValidateJWT(authToken, cfg.jwtToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	hashedPassword, err := auth.HashPassword(data.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	UserParam := database.UpdateUsersParams{
		Email:    data.Email,
		Password: hashedPassword,
		ID:       userID,
	}

	err = cfg.dbQueries.UpdateUsers(req.Context(), UserParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error updating the user")
		return
	}

	respondWithJSON(w, http.StatusOK, User{
		ID:    userID,
		Email: data.Email,
	})
}

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, req *http.Request) {
	authToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userID, err := auth.ValidateJWT(authToken, cfg.jwtToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	chirpID, err := uuid.Parse(req.PathValue("chirpID"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(req.Context(), chirpID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if userID != chirp.UserID {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	err = cfg.dbQueries.DeleteChirp(req.Context(), chirpID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) polkaWebhooksHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	apiKey, err := auth.GetAPIKey(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if cfg.polkaKey != apiKey {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	data := webhooksRequest{}
	err = json.NewDecoder(req.Body).Decode(&data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	if data.Event != "user.upgraded" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	_, err = cfg.dbQueries.CheckUser(req.Context(), data.Data.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = cfg.dbQueries.SubscribeUser(req.Context(), data.Data.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
