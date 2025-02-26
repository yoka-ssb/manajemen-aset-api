package services

import (
	"asset-management-api/app/auth"
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type UserService struct {
	MasterService
	assetpb.UnimplementedUSERServiceServer
	DB *pgxpool.Pool
}

type User struct {
	Nip          int32  `json:"nip"`
	UserFullName string `json:"user_full_name"`
	UserEmail    string `json:"user_email"`
	UserPassword string `json:"user_password"`
	RoleID       int32  `json:"role_id"`
	AreaID       *int32 `json:"area_id"`
	OutletID     *int32 `json:"outlet_id"`
}

func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{
		DB: db,
	}
}

func (s *UserService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterUSERServiceServer(grpcServer, s)
}

func (s *UserService) CreateUser(ctx context.Context, req *assetpb.CreateUserRequest) (*assetpb.CreateUserResponse, error) {
	if req == nil {
		return &assetpb.CreateUserResponse{
			Message: "Missing request body",
			Code:    "400",
			Success: false}, nil
	}
	log.Info().Msg("Creating new user")

	hashedPassword, err := utils.HashPassword(req.GetUserPassword())
	if err != nil {
		return &assetpb.CreateUserResponse{
			Message: err.Error(),
			Code:    "400",
			Success: false}, nil
	}

	var areaId *int32
	if req.AreaId != 0 {
		areaId = &req.AreaId
	}

	var outletId *int32
	if req.OutletId != 0 {
		outletId = &req.OutletId
	}

	query := `INSERT INTO users (nip, user_full_name, user_email, user_password, role_id, area_id, outlet_id) 
              VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = s.DB.Exec(ctx, query, req.GetNip(), req.GetUserFullName(), req.GetUserEmail(), hashedPassword, req.GetRoleId(), areaId, outletId)
	if err != nil {
		return &assetpb.CreateUserResponse{
			Message: err.Error(),
			Code:    "400",
			Success: false}, nil
	}

	log.Info().Msgf("New user created with NIP: %d", req.GetNip())

	return &assetpb.CreateUserResponse{
		Message: "Successfully created user",
		Code:    "200",
		Success: true}, nil
}
func (s *UserService) GetUser(ctx context.Context, req *assetpb.GetUserRequest) (*assetpb.GetUserResponse, error) {
	log.Info().Msgf("Getting user with nip: %d", req.GetNip())
	var user assetpb.User
	var areaID sql.NullInt32
	var outletID sql.NullInt32

	query := `SELECT users.nip, users.user_full_name, users.user_email, users.role_id, users.area_id, users.outlet_id, roles.role_name 
              FROM users 
              LEFT JOIN roles ON users.role_id = roles.role_id 
              WHERE users.nip = $1 LIMIT 1`
	row := s.DB.QueryRow(ctx, query, req.GetNip())

	err := row.Scan(&user.Nip, &user.UserFullName, &user.UserEmail, &user.RoleId, &areaID, &outletID, &user.RoleName)
	if err != nil {
		log.Error().Err(err).Msg("User not found")
		return &assetpb.GetUserResponse{
			Message: "User not found",
			Code:    "400",
			Success: false}, nil
	}

	// Convert NULL values to default values
	if areaID.Valid {
		user.AreaId = areaID.Int32
	} else {
		user.AreaId = 0
	}

	if outletID.Valid {
		user.OutletId = outletID.Int32
	} else {
		user.OutletId = 0
	}

	return &assetpb.GetUserResponse{
		Message: "Successfully fetched user",
		Code:    "200",
		Data:    &user,
		Success: true,
	}, nil
}
func (s *UserService) UpdateUser(ctx context.Context, req *assetpb.UpdateUserRequest) (*assetpb.UpdateUserResponse, error) {
	log.Info().Msgf("Updating user with nip: %d", req.GetNip())

	query := `UPDATE users SET user_full_name = $1, user_email = $2, role_id = $3`
	params := []interface{}{req.GetUserFullName(), req.GetUserEmail(), req.GetRoleId()}

	if req.GetAreaId() > 0 {
		query += ", area_id = $4"
		params = append(params, req.GetAreaId())
	}
	if req.GetOutletId() > 0 {
		query += ", outlet_id = $5"
		params = append(params, req.GetOutletId())
	}
	query += " WHERE nip = $6"
	params = append(params, req.GetNip())

	_, err := s.DB.Exec(ctx, query, params...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return &assetpb.UpdateUserResponse{
			Message: "Failed to update user: " + err.Error(),
			Code:    "400",
			Success: false,
		}, nil
	}

	return &assetpb.UpdateUserResponse{
		Message: "Successfully updated user",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *assetpb.DeleteUserRequest) (*assetpb.DeleteUserResponse, error) {
	log.Info().Msg("Deleting user")
	query := "DELETE FROM users WHERE nip = $1"
	result, err := s.DB.Exec(ctx, query, req.GetNip())
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete user")
		return &assetpb.DeleteUserResponse{Success: false}, nil
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warn().Msg("No user found to delete")
		return &assetpb.DeleteUserResponse{Success: false}, nil
	}

	return &assetpb.DeleteUserResponse{Success: true}, nil
}

func (s *UserService) ListUsers(ctx context.Context, req *assetpb.ListUsersRequest) (*assetpb.ListUsersResponse, error) {
	log.Info().Msg("Listing users")
	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	offset := (pageNumber - 1) * pageSize

	query := `SELECT nip, user_full_name, user_email, role_id, area_id, outlet_id FROM users`
	var params []interface{}

	if q != "" {
		query += " WHERE user_full_name LIKE $1 OR user_email LIKE $2"
		params = append(params, "%"+q+"%", "%"+q+"%")
	}

	query += " ORDER BY nip ASC LIMIT $3 OFFSET $4"
	params = append(params, pageSize, offset)

	if q == "" {
		query = `SELECT nip, user_full_name, user_email, role_id, area_id, outlet_id FROM users ORDER BY nip ASC LIMIT $1 OFFSET $2`
		params = []interface{}{pageSize, offset}
	}

	rows, err := s.DB.Query(ctx, query, params...)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching users")
		return nil, err
	}
	defer rows.Close()

	var users []*assetpb.User
	for rows.Next() {
		var user assetpb.User
		var areaID sql.NullInt32
		var outletID sql.NullInt32

		err := rows.Scan(&user.Nip, &user.UserFullName, &user.UserEmail, &user.RoleId, &areaID, &outletID)

		// Konversi nilai NULL ke default (misalnya 0)
		if areaID.Valid {
			user.AreaId = areaID.Int32
		} else {
			user.AreaId = 0 // Atur default value jika NULL
		}

		if outletID.Valid {
			user.OutletId = outletID.Int32
		} else {
			user.OutletId = 0 // Atur default value jika NULL
		}

		if err != nil {
			log.Error().Err(err).Msg("Error scanning user row")
			return nil, err
		}
		users = append(users, &user)
	}

	var totalCount int32
	countQuery := "SELECT COUNT(*) FROM users"
	err = s.DB.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching total count")
		return nil, err
	}

	resp := &assetpb.ListUsersResponse{
		Data:       users,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if totalCount > offset+pageSize {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	return resp, nil
}

func (s *UserService) ResetPassword(ctx context.Context, req *assetpb.ResetPasswordRequest) (*assetpb.ResetPasswordResponse, error) {
	log.Info().Msg("Resetting password")

	// Validate user
	var userCount int
	err := s.DB.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE nip = $1", req.GetNip()).Scan(&userCount)
	if err != nil || userCount == 0 {
		log.Error().Err(err).Msg("User not found")
		return &assetpb.ResetPasswordResponse{
			Message: "User not found",
			Code:    "404",
			Success: false}, nil
	}

	// Validate token
	token := auth.ValidateToken(req.GetResetToken())
	if token == nil {
		log.Error().Msg("Invalid reset token")
		return &assetpb.ResetPasswordResponse{
			Message: "Invalid reset token",
			Code:    "400",
			Success: false}, nil
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.GetUserPassword())
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return &assetpb.ResetPasswordResponse{
			Message: "Failed to hash password",
			Code:    "400",
			Success: false}, nil
	}

	// Reset password: update user password
	_, err = s.DB.Exec(ctx, "UPDATE users SET user_password = $1 WHERE nip = $2", hashedPassword, req.GetNip())
	if err != nil {
		log.Error().Err(err).Msg("Failed to reset password")
		return &assetpb.ResetPasswordResponse{
			Message: "Failed to reset password",
			Code:    "400",
			Success: false}, nil
	}

	return &assetpb.ResetPasswordResponse{
		Message: "Successfully reset password",
		Code:    "200",
		Success: true}, nil
}
func getUsers(db *pgxpool.Pool, offset, limit int32, q string) ([]*assetpb.User, error) {
	var users []*assetpb.User
	var rows pgx.Rows
	var err error

	query := "SELECT users.nip, users.user_full_name, users.user_email, users.user_password, users.role_id, users.area_id, users.outlet_id, roles.role_name FROM users LEFT JOIN roles ON users.role_id = roles.role_id"
	params := []interface{}{}

	if q != "" {
		query += " WHERE users.user_full_name LIKE $1"
		params = append(params, "%"+q+"%")
	}

	query += " ORDER BY users.nip ASC LIMIT $2 OFFSET $3"
	params = append(params, limit, offset)

	rows, err = db.Query(context.Background(), query, params...)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching users")
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user assetpb.User
		var areaID sql.NullInt32
		var outletID sql.NullInt32

		err := rows.Scan(&user.Nip, &user.UserFullName, &user.UserEmail, &user.UserPassword, &user.RoleId, &areaID, &outletID, &user.RoleName)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning user row")
			return nil, err
		}

		// Convert NULL values to default values
		if areaID.Valid {
			user.AreaId = areaID.Int32
		} else {
			user.AreaId = 0
		}

		if outletID.Valid {
			user.OutletId = outletID.Int32
		} else {
			user.OutletId = 0
		}

		users = append(users, &user)
	}
	return users, nil
}
