package services

import (
	"asset-management-api/app/auth"
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type AuthService struct {
	db *pgxpool.Pool
	assetpb.UnimplementedAUTHServiceServer
}

func NewAuthService(db *pgxpool.Pool) *AuthService {
	return &AuthService{db: db}
}

func (s *AuthService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterAUTHServiceServer(grpcServer, s)
}

func (s *AuthService) tokenStore(tokenString string) error {
	if tokenString == "" {
		return errors.New("invalid token: empty string")
	}

	expirationTime := time.Now().Add(72 * time.Hour) // Token expires in 3 days
	query := `INSERT INTO token_stores (token, created_at, exp_token) VALUES ($1, NOW(), $2)`

	_, err := s.db.Exec(context.Background(), query, tokenString, expirationTime)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store token")
		return err
	}

	log.Info().Msgf("Token stored successfully with ExpToken: %s", expirationTime)
	return nil
}

func (s *AuthService) deleteToken(tokenString string) error {
	query := `DELETE FROM token_stores WHERE token = $1`
	result, err := s.db.Exec(context.Background(), query, tokenString)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete token")
		return err
	}

	rowsAffected := result.RowsAffected()
	log.Info().Msgf("Token deleted, affected rows: %d", rowsAffected)
	return nil
}

func (s *AuthService) GetToken(tokenString string) *string {
	query := `SELECT token FROM token_stores WHERE token = $1 LIMIT 1`
	var token string

	err := s.db.QueryRow(context.Background(), query, tokenString).Scan(&token)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Msg("Token not found")
			return nil
		}
		log.Error().Err(err).Msg("Error retrieving token")
		return nil
	}

	return &token
}

func (s *AuthService) Login(ctx context.Context, req *assetpb.LoginRequest) (*assetpb.LoginResponse, error) {
	log.Info().Msgf("Logging in user with NIP: %s", req.GetNip())

	// Get user by NIP
	query := `SELECT nip, user_password FROM users WHERE nip = $1 LIMIT 1`
	var storedNIP, storedPassword string

	err := s.db.QueryRow(context.Background(), query, req.GetNip()).Scan(&storedNIP, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Msg("User not found")
			return nil, status.Errorf(http.StatusNotFound, "User not found")
		}
		log.Error().Err(err).Msg("Error retrieving user")
		return nil, status.Errorf(http.StatusInternalServerError, "Error retrieving user")
	}

	// Verify password
	err = utils.VerifyPassword(storedPassword, req.GetUserPassword())
	if err != nil {
		log.Warn().Msg("Invalid password attempt")
		return nil, status.Errorf(http.StatusBadRequest, "Invalid password")
	}

	// Generate token
	nipInt, err := strconv.Atoi(storedNIP)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert NIP to int")
		return nil, status.Errorf(http.StatusInternalServerError, "Failed to convert NIP to int")
	}
	token := auth.GenerateJWTToken(int32(nipInt))

	if token == nil {
		log.Error().Msg("Token generation failed")
		return nil, status.Errorf(http.StatusInternalServerError, "Failed to generate token")
	}

	// Save token to database
	err = s.tokenStore(*token)
	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "Failed to save token")
	}

	return &assetpb.LoginResponse{
		Message: "Successfully logged in",
		Code:    "200",
		Token:   *token,
		Success: true,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *assetpb.LogoutRequest) (*assetpb.LogoutResponse, error) {
	log.Info().Msgf("Logging out user with token: %s", req.Token)

	// Delete token
	err := s.deleteToken(req.Token)
	if err != nil {
		return &assetpb.LogoutResponse{
			Message: "Failed to logout",
			Code:    "400",
			Success: false,
		}, nil
	}

	return &assetpb.LogoutResponse{
		Message: "Successfully logged out",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *AuthService) deleteExpiredTokens() {
	currentTime := time.Now()
	log.Info().Msgf("Running deleteExpiredTokens at: %s", currentTime)

	// Delete expired tokens
	query := `DELETE FROM token_stores WHERE exp_token < $1`
	result, err := s.db.Exec(context.Background(), query, currentTime)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete expired tokens")
		return
	}

	rowsAffected := result.RowsAffected()

	log.Info().Msgf("Deleted expired tokens, affected rows: %d", rowsAffected)
}
