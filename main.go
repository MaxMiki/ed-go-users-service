package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	defaultMongoURI = "mongodb://localhost:27017"
	databaseName    = "usersdb"
	collectionName  = "users"
)

// User represents a user document stored in MongoDB.
type User struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string        `bson:"name" json:"name"`
	Email     string        `bson:"email" json:"email"`
	CreatedAt time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time     `bson:"updated_at" json:"updatedAt"`
}

// Server holds application dependencies.
type Server struct {
	collection *mongo.Collection
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client, err := connectDB(ctx)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		if err := client.Disconnect(disconnectCtx); err != nil {
			log.Printf("failed to disconnect from MongoDB: %v", err)
		}
	}()

	srv := &Server{collection: client.Database(databaseName).Collection(collectionName)}

	mux := http.NewServeMux()
	mux.HandleFunc("/users/", srv.handleUserByID)
	mux.HandleFunc("/users", srv.handleUsers)
	mux.HandleFunc("/health", handleHealth)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
	log.Println("users service listening on http://localhost:8080")

	<-ctx.Done()
	log.Println("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

// connectDB creates a MongoDB client from the MONGODB_URI environment variable.
func connectDB(ctx context.Context) (*mongo.Client, error) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = defaultMongoURI
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, err
	}
	return client, nil
}

// handleUsers routes requests for the /users collection.
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listUsers(w, r)
	case http.MethodPost:
		s.createUser(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleUserByID routes requests for a specific user resource.
func (s *Server) handleUserByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/users/")
	if id == "" || strings.Contains(id, "/") {
		respondError(w, http.StatusNotFound, "not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getUser(w, r, id)
	case http.MethodPut:
		s.updateUser(w, r, id)
	case http.MethodDelete:
		s.deleteUser(w, r, id)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// createUser inserts a new user document.
func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Name == "" || req.Email == "" {
		respondError(w, http.StatusBadRequest, "name and email are required")
		return
	}

	now := time.Now().UTC()
	user := User{
		ID:        bson.NewObjectID(),
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if _, err := s.collection.InsertOne(ctx, user); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create user: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

// listUsers returns all users sorted by creation time.
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cursor, err := s.collection.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list users: %v", err))
		return
	}
	defer cursor.Close(ctx)

	users := []User{}
	if err := cursor.All(ctx, &users); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to decode users: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, users)
}

// getUser returns a single user by ID.
func (s *Server) getUser(w http.ResponseWriter, r *http.Request, id string) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user User
	if err := s.collection.FindOne(ctx, bson.D{{Key: "_id", Value: objectID}}).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get user: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// updateUser modifies an existing user's name and email.
func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, id string) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Name == "" || req.Email == "" {
		respondError(w, http.StatusBadRequest, "name and email are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "_id", Value: objectID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "name", Value: req.Name},
			{Key: "email", Value: req.Email},
			{Key: "updated_at", Value: time.Now().UTC()},
		}},
	}

	if _, err := s.collection.UpdateOne(ctx, filter, update); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update user: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteUser removes a user by ID.
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, id string) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if _, err := s.collection.DeleteOne(ctx, bson.D{{Key: "_id", Value: objectID}}); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete user: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleHealth reports whether the service is running.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
