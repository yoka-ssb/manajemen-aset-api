package services

import (
	"asset-management-api/app/auth"
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type TokenStore struct {
	Token     string    `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type AuthService struct {
	MasterService
	assetpb.UnimplementedAUTHServiceServer
}

func NewAuthService(db *gorm.DB) *AuthService {
	authService := &AuthService{
		MasterService: MasterService{DB: db},
	}

	// Start the background job to delete expired tokens
	go func() {
		for {
			deleteExpiredTokens(db)
			time.Sleep(24 * time.Hour) // Run the job every 24 hours
		}
	}()

	return authService
}

func (s *AuthService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterAUTHServiceServer(grpcServer, s)
}

func (s *AuthService) tokenStore(tokenString string) error {
	var TokenStore TokenStore
	// Save token to database
	TokenStore.Token = tokenString
	TokenStore.CreatedAt = time.Now()
	err := s.DB.Create(&TokenStore).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *AuthService) deleteToken(tokenString string) error {
	// Delete token from database
	err := s.DB.Where("token = ?", tokenString).Delete(&assetpb.TokenStore{}).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *AuthService) GetToken(tokenString string) *string {
	// Get token from database
	var tokenStore assetpb.TokenStore
	result := s.DB.Where("token = ?", tokenString).First(&tokenStore)
	if result.Error != nil {
		return nil
	}
	return &tokenStore.Token
}

func (s *AuthService) Login(ctx context.Context, req *assetpb.LoginRequest) (*assetpb.LoginResponse, error) {
	log.Info().Msg("Logging in")

	// Getting user by nip
	var user assetpb.User
	err := s.DB.Where("nip = ?", req.GetNip()).First(&user).Error
	if err != nil {
		return nil, status.Errorf(http.StatusNotFound, "User not found")
	}

	// Verify password
	err = utils.VerifyPassword(user.GetUserPassword(), req.GetUserPassword())
	if err != nil {
		return nil, status.Errorf(http.StatusBadRequest, "Invalid password")
	}

	// Generate token
	token := auth.GenerateJWTToken(user.GetNip())

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
	log.Info().Msg("Logging Out")

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

func deleteExpiredTokens(db *gorm.DB) {
	// Calculate the expiration time
	expirationTime := time.Now().Add(-72 * time.Hour)

	// Delete tokens older than the expiration time
	result := db.Where("created_at < ?", expirationTime).Delete(&assetpb.TokenStore{})
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Failed to update submission status")
	} else {
		log.Info().Int64("rowsAffected", result.RowsAffected).Msg("Deleted expired tokens")
	}
}
