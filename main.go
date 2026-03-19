package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/chirpy/internal/auth"
	"github.com/chirpy/internal/database"
	"github.com/chirpy/utils"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int64
	dbQueries      *database.Queries
	Platform       string
	SecretToken    string
	PolkaKey       string
}

func main() {
	godotenv.Load()
	dbUrl := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatalf("DB connection failed. DB URL: %s", dbUrl)
	}
	db.Ping()
	port := "8080"
	log.Printf("Starting server on port %s", port)
	serverMux := http.ServeMux{}
	server := http.Server{
		Addr:    ":" + port,
		Handler: &serverMux,
	}
	c := &apiConfig{}
	c.dbQueries = database.New(db)
	defer db.Close()
	c.Platform = os.Getenv("PLATFORM")
	c.SecretToken = os.Getenv("SECRET_TOKEN")
	c.PolkaKey = os.Getenv("POLKA_KEY")
	handler := http.StripPrefix("/app", http.FileServer(http.Dir("./")))
	serverMux.Handle("/app/", c.middlewareMetricsInc(handler))
	serverMux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	serverMux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, c.fileServerHits.Load())))
	})
	serverMux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if c.Platform != "dev" {
			http.Error(w, "Not allowed", http.StatusForbidden)
			return
		}
		c.fileServerHits.Store(0)
		err := c.dbQueries.DeleteAllUsers(r.Context())
		if err != nil {
			http.Error(w, "failed to reset", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Metrics reset"))
	})
	// serverMux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
	// 	type parameters struct {
	// 		Body string `json:"body"`
	// 	}
	// 	decoder := json.NewDecoder(r.Body)
	// 	var params parameters
	// 	err := decoder.Decode(&params)
	// 	if err != nil {
	// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
	// 		return
	// 	}
	// 	if len(params.Body) == 0 {
	// 		http.Error(w, "Chirp body cannot be empty", http.StatusBadRequest)
	// 		return
	// 	}
	// 	if len(params.Body) > 140 {
	// 		http.Error(w, "Chirp body cannot exceed 140 characters", http.StatusBadRequest)
	// 		return
	// 	}
	// 	replaced_body := utils.ReplaceBadWords(params.Body)
	// 	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 	w.WriteHeader(http.StatusOK)
	// 	type response struct {
	// 		Cleaned_Body string `json:"cleaned_body"`
	// 		Valid        bool   `json:"valid"`
	// 	}
	// 	resp := response{Cleaned_Body: replaced_body, Valid: true}
	// 	data, err := json.Marshal(&resp)
	// 	if err != nil {
	// 		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
	// 		return
	// 	}
	// 	w.Write(data)
	// })
	serverMux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var param parameters
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&param)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if param.Email == "" {
			http.Error(w, "email missing", http.StatusBadRequest)
			return
		}
		hashedPassword := auth.HashedPassword(param.Password)

		user := database.CreateUserParams{
			ID:             uuid.New(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			Email:          param.Email,
			HashedPassword: hashedPassword,
		}
		createdUser, err := c.dbQueries.CreateUser(r.Context(), user)
		if err != nil {
			msg := fmt.Sprintf("Failed to create user. Error: %v", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		type response struct {
			ID          uuid.UUID `json:"id"`
			CreatedAt   time.Time `json:"created_at"`
			UpdatedAt   time.Time `json:"updated_at"`
			Email       string    `json:"email"`
			IsChirpyRed bool      `json:"is_chirpy_red"`
		}
		resp := response{
			ID:          createdUser.ID,
			CreatedAt:   createdUser.CreatedAt,
			UpdatedAt:   createdUser.UpdatedAt,
			Email:       createdUser.Email,
			IsChirpyRed: createdUser.IsChirpyRed,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Something went wrong.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write(data)
	})
	serverMux.HandleFunc("PUT /api/users", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := auth.ValidateJWT(token, c.SecretToken)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var param parameters
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&param)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		hashedPassword := auth.HashedPassword(param.Password)
		user, err := c.dbQueries.UpdateUser(r.Context(), database.UpdateUserParams{
			ID:             userID,
			UpdatedAt:      time.Now(),
			Email:          param.Email,
			HashedPassword: hashedPassword,
		})
		if err != nil {
			msg := fmt.Sprintf("Failed to update user. Error: %v", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		type response struct {
			ID          uuid.UUID `json:"id"`
			CreatedAt   time.Time `json:"created_at"`
			UpdatedAt   time.Time `json:"updated_at"`
			Email       string    `json:"email"`
			IsChirpyRed bool      `json:"is_chirpy_red"`
		}
		resp := response{
			ID:          user.ID,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
			Email:       user.Email,
			IsChirpyRed: user.IsChirpyRed,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Something went wrong.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	serverMux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		bearerToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := auth.ValidateJWT(bearerToken, c.SecretToken)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		type reqParams struct {
			Body string `json:"body"`
		}
		decoder := json.NewDecoder(r.Body)
		params := reqParams{}
		err = decoder.Decode(&params)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		replacedBody, err := utils.ValidateChirp(params.Body)
		if err != nil {
			http.Error(w, "Failed to validate chirp", http.StatusInternalServerError)
			return
		}
		user, err := c.dbQueries.GetUserById(r.Context(), userID)
		if err != nil {
			http.Error(w, "Failed to validate user", http.StatusBadRequest)
			return
		}
		chirp := database.CreateChirpParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Body:      replacedBody,
			UserID:    user.ID,
		}
		rec, err := c.dbQueries.CreateChirp(r.Context(), chirp)
		if err != nil {
			http.Error(w, "Failed to create chirp", http.StatusInternalServerError)
			return
		}
		type response struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}
		resp := response{
			ID:        rec.ID,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
			Body:      rec.Body,
			UserID:    rec.UserID,
		}
		data, err := json.Marshal(&resp)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write(data)
	})
	serverMux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		authorId := r.URL.Query().Get("author_id")
		order := r.URL.Query().Get("sort")
		if order != "" && order != "asc" && order != "desc" {
			http.Error(w, "Invalid sort parameter", http.StatusBadRequest)
			return
		}
		if order == "" {
			order = "asc"
		}
		var chirps []database.Chirp
		if authorId != "" {
			authorIdUUID, err := uuid.Parse(authorId)
			if err != nil {
				http.Error(w, "Invalid author_id", http.StatusBadRequest)
				return
			}
			chirps, err = c.dbQueries.GetChirpsByAuthorIdSort(r.Context(), authorIdUUID)
			if err != nil {
				http.Error(w, "Failed to fetch chirps", http.StatusInternalServerError)
				return
			}
			type response struct {
				ID        uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Body      string    `json:"body"`
				UserID    uuid.UUID `json:"user_id"`
			}
			resp := make([]response, len(chirps))
			for i, chirp := range chirps {
				resp[i] = response{
					ID:        chirp.ID,
					CreatedAt: chirp.CreatedAt,
					UpdatedAt: chirp.UpdatedAt,
					Body:      chirp.Body,
					UserID:    chirp.UserID,
				}
			}
			data, err := json.Marshal(&resp)
			if err != nil {
				http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}
		chirps, err = c.dbQueries.GetAllChirps(r.Context())
		sort.Slice(chirps, func(i, j int) bool {
			if order == "asc" {
				return chirps[i].CreatedAt.Before(chirps[j].CreatedAt)
			} else {
				return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
			}
		})
		if err != nil {
			http.Error(w, "Failed to fetch chirps", http.StatusInternalServerError)
			return
		}
		type response struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}
		resp := make([]response, len(chirps))
		for i, chirp := range chirps {
			resp[i] = response{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID,
			}
		}
		data, err := json.Marshal(&resp)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	serverMux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpID := r.PathValue("chirpID")
		chirpUUID, err := uuid.Parse(chirpID)
		if err != nil {
			http.Error(w, "Invalid chirp ID", http.StatusBadRequest)
			return
		}
		chirp, err := c.dbQueries.GetChirpById(r.Context(), chirpUUID)
		if err == sql.ErrNoRows {
			http.Error(w, "Chirp not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to fetch chirp", http.StatusInternalServerError)
			return
		}
		type response struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}
		resp := response{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		}
		data, err := json.Marshal(&resp)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	serverMux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var param parameters
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&param)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if param.Email == "" {
			http.Error(w, "email missing", http.StatusBadRequest)
			return
		}
		user, err := c.dbQueries.GetUserByEmail(r.Context(), param.Email)
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
			return
		}
		match := auth.ComparePasswordAndHash(param.Password, user.HashedPassword)
		if !match {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		accessToken, err := auth.MakeJWT(user.ID, c.SecretToken, 3600*time.Second)
		if err != nil {
			http.Error(w, "Failed to create token", http.StatusInternalServerError)
			return
		}
		type response struct {
			ID           uuid.UUID `json:"id"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
			Email        string    `json:"email"`
			RefreshToken string    `json:"refresh_token"`
			Token        string    `json:"token"`
			IsChirpyRed  bool      `json:"is_chirpy_red"`
		}
		refreshToken := auth.MakeRefreshToken()
		_, err = c.dbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
			Token:     refreshToken,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
			UserID:    user.ID,
		})
		if err != nil {
			http.Error(w, "Failed to create refresh token", http.StatusInternalServerError)
			return
		}
		resp := response{
			ID:           user.ID,
			CreatedAt:    user.CreatedAt,
			UpdatedAt:    user.UpdatedAt,
			Email:        user.Email,
			RefreshToken: refreshToken,
			Token:        string(accessToken),
			IsChirpyRed:  user.IsChirpyRed,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Something went wrong.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	serverMux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		refreshTokenRecord, err := c.dbQueries.GetRefreshTokenByToken(r.Context(), refreshToken)
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Failed to fetch refresh token", http.StatusInternalServerError)
			return
		}
		if time.Now().After(refreshTokenRecord.ExpiresAt) {
			http.Error(w, "Refresh token expired", http.StatusUnauthorized)
			return
		}
		user, err := c.dbQueries.GetUserById(r.Context(), refreshTokenRecord.UserID)
		if err != nil {
			http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
			return
		}
		accessToken, err := auth.MakeJWT(user.ID, c.SecretToken, 3600*time.Second)
		if err != nil {
			http.Error(w, "Failed to create token", http.StatusInternalServerError)
			return
		}
		type response struct {
			Token string `json:"token"`
		}
		resp := response{
			Token: string(accessToken),
		}
		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Something went wrong.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	serverMux.HandleFunc("POST /api/revoke", func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		err = c.dbQueries.RevokeRefreshToken(r.Context(), refreshToken)
		if err != nil {
			http.Error(w, "Failed to revoke refresh token", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	serverMux.HandleFunc("DELETE /api/chirps/{chirpId}", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID, err := auth.ValidateJWT(token, c.SecretToken)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		chirpID := r.PathValue("chirpId")
		chirpUUID, err := uuid.Parse(chirpID)
		if err != nil {
			http.Error(w, "Invalid chirp ID", http.StatusBadRequest)
			return
		}
		chirp, err := c.dbQueries.GetChirpById(r.Context(), chirpUUID)
		if err == sql.ErrNoRows {
			http.Error(w, "Chirp not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to fetch chirp", http.StatusInternalServerError)
			return
		}
		if chirp.UserID != userID {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		err = c.dbQueries.DeleteChirpById(r.Context(), chirpUUID)
		if err != nil {
			http.Error(w, "Failed to delete chirp", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	serverMux.HandleFunc("POST /api/polka/webhooks", func(w http.ResponseWriter, r *http.Request) {
		apiKey, err := auth.GetAPIKey(r.Header)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if apiKey != c.PolkaKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		type parameters struct {
			Event string `json:"event"`
			Data  struct {
				UserID string `json:"user_id"`
			} `json:"data"`
		}
		var param parameters
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&param)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if param.Event == "user.upgraded" {
			userID, err := uuid.Parse(param.Data.UserID)
			if err != nil {
				http.Error(w, "Invalid user ID", http.StatusBadRequest)
				return
			}
			user, err := c.dbQueries.UpgradeUserToChirpyRed(r.Context(), database.UpgradeUserToChirpyRedParams{
				ID:        userID,
				UpdatedAt: time.Now(),
			})
			if err == sql.ErrNoRows {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
			if err != nil {
				http.Error(w, "Failed to upgrade user", http.StatusInternalServerError)
				return
			}
			type response struct {
				ID          uuid.UUID `json:"id"`
				CreatedAt   time.Time `json:"created_at"`
				UpdatedAt   time.Time `json:"updated_at"`
				Email       string    `json:"email"`
				IsChirpyRed bool      `json:"is_chirpy_red"`
			}
			resp := response{
				ID:          user.ID,
				CreatedAt:   user.CreatedAt,
				UpdatedAt:   user.UpdatedAt,
				Email:       user.Email,
				IsChirpyRed: user.IsChirpyRed,
			}
			data, err := json.Marshal(resp)
			if err != nil {
				http.Error(w, "Something went wrong.", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNoContent)
			w.Write(data)
		} else { // ignore all other event types
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})
	log.Fatal(server.ListenAndServe())
}

func (c *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
