package services

import (
	"asset-management-api/assetpb"
	"context"
	"errors"
	"fmt"
	"time"

	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NotificationService struct {
	DB *pgxpool.Pool
	assetpb.UnimplementedNOTIFICATIONServiceServer
}

func NewNotificationService(db *pgxpool.Pool) *NotificationService {
	return &NotificationService{DB: db}
}

func (s *NotificationService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterNOTIFICATIONServiceServer(grpcServer, s)
}

func (s *NotificationService) InsertNotificationsForAllAssets(ctx context.Context, req *assetpb.InsertAllRequest) (*assetpb.InsertAllResponse, error) {
	log.Info().Msg("Processing notifications based on asset maintenance dates and submissions")

	// Step 1: Fetch assets from database
	query := `SELECT asset_id, asset_name, asset_maintenance_date, outlet_id, area_id FROM assets`
	rows, err := s.DB.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to retrieve assets")
		return nil, err
	}
	defer rows.Close()

	var assets []struct {
		AssetId              int32
		AssetName            string
		AssetMaintenanceDate time.Time
		OutletId             int32
		AreaId               int32
	}

	for rows.Next() {
		var asset struct {
			AssetId              int32
			AssetName            string
			AssetMaintenanceDate time.Time
			OutletId             int32
			AreaId               int32
		}
		if err := rows.Scan(&asset.AssetId, &asset.AssetName, &asset.AssetMaintenanceDate, &asset.OutletId, &asset.AreaId); err != nil {
			log.Error().Err(err).Msg("Failed to scan asset row")
			continue
		}
		assets = append(assets, asset)
	}

	log.Info().Msgf("Found %d assets", len(assets))

	// Step 2: Process assets
	for _, asset := range assets {
		log.Info().Msgf("Processing asset ID: %d, Name: %s, Maintenance Date: %s", asset.AssetId, asset.AssetName, asset.AssetMaintenanceDate.Format("2006-01-02"))

		daysUntilMaintenance := int(time.Until(asset.AssetMaintenanceDate).Hours() / 24)
		notificationStatus := "normal"
		if daysUntilMaintenance < 0 {
			notificationStatus = "late"
		} else if daysUntilMaintenance <= 7 {
			notificationStatus = "waiting"
		}

		// Check if notification exists
		var existingNotificationId int
		err = s.DB.QueryRow(ctx, `SELECT id_notification FROM notifications WHERE asset_id = $1`, asset.AssetId).Scan(&existingNotificationId)

		if errors.Is(err, sql.ErrNoRows) {
			if notificationStatus != "normal" {
				log.Info().Msgf("Creating notification for asset ID %d", asset.AssetId)
				_, err := s.DB.Exec(ctx, `INSERT INTO notifications (asset_id, asset_name, outlet_id, area_id, maintenance_or_submitted, status) VALUES ($1, $2, $3, $4, $5, $6)`,
					asset.AssetId, asset.AssetName, asset.OutletId, asset.AreaId, asset.AssetMaintenanceDate.Format("2006-01-02"), notificationStatus)
				if err != nil {
					log.Error().Err(err).Msgf("Failed to insert notification for asset ID %d", asset.AssetId)
				}
			}
		} else if err != nil {
			log.Error().Err(err).Msgf("Error checking notification for asset ID %d", asset.AssetId)
		} else {
			if notificationStatus == "normal" {
				log.Info().Msgf("Deleting notification for asset ID %d", asset.AssetId)
				_, err := s.DB.Exec(ctx, `DELETE FROM notifications WHERE asset_id = $1`, asset.AssetId)
				if err != nil {
					log.Error().Err(err).Msgf("Failed to delete notification for asset ID %d", asset.AssetId)
				}
			} else {
				log.Info().Msgf("Updating notification for asset ID %d", asset.AssetId)
				_, err := s.DB.Exec(ctx, `UPDATE notifications SET maintenance_or_submitted = $1, status = $2 WHERE asset_id = $3`,
					asset.AssetMaintenanceDate.Format("2006-01-02"), notificationStatus, asset.AssetId)
				if err != nil {
					log.Error().Err(err).Msgf("Failed to update notification for asset ID %d", asset.AssetId)
				}
			}
		}
	}

	// Step 3: Fetch submissions
	query = `SELECT submission_id, asset_id, submission_asset_name, outlet_id, area_id FROM submissions`
	rows, err = s.DB.Query(ctx, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to retrieve submissions")
		return nil, err
	}
	defer rows.Close()

	var submissions []struct {
		SubmissionId        *int32
		AssetId             int32
		SubmissionAssetName string
		OutletId            int32
		AreaId              int32
	}

	for rows.Next() {
		var submission struct {
			SubmissionId        *int32
			AssetId             int32
			SubmissionAssetName string
			OutletId            int32
			AreaId              int32
		}
		if err := rows.Scan(&submission.SubmissionId, &submission.AssetId, &submission.SubmissionAssetName, &submission.OutletId, &submission.AreaId); err != nil {
			log.Error().Err(err).Msg("Failed to scan submission row")
			continue
		}
		submissions = append(submissions, submission)
	}

	log.Info().Msgf("Found %d submissions", len(submissions))

	// Step 4: Process submissions
	for _, submission := range submissions {
		log.Info().Msgf("Processing submission ID: %d, Asset ID: %d", submission.SubmissionId, submission.AssetId)

		var existingNotificationId int
		err = s.DB.QueryRow(ctx, `SELECT id_notification FROM notifications WHERE asset_id = $1 AND status = 'submitted'`, submission.AssetId).Scan(&existingNotificationId)

		if errors.Is(err, sql.ErrNoRows) {
			log.Info().Msgf("Creating notification for submission ID %d", submission.SubmissionId)
			_, err := s.DB.Exec(ctx, `INSERT INTO notifications (asset_id, submission_id, asset_name, outlet_id, area_id, maintenance_or_submitted, status) VALUES ($1, $2, $3, $4, $5, $6, 'submitted')`,
				submission.AssetId, submission.SubmissionId, submission.SubmissionAssetName, submission.OutletId, submission.AreaId, time.Now().Format("2006-01-02"))
			if err != nil {
				log.Error().Err(err).Msgf("Failed to insert notification for submission ID %d", submission.SubmissionId)
			}
		} else if err != nil {
			log.Error().Err(err).Msgf("Error checking notification for submission ID %d", submission.SubmissionId)
		} else {
			log.Info().Msgf("Notification for submission ID %d already exists, skipping", submission.SubmissionId)
		}
	}

	return &assetpb.InsertAllResponse{
		Message: "Notifications processed successfully",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *NotificationService) PostNotifications(ctx context.Context, req *assetpb.InsertAllRequest) (*assetpb.InsertAllResponse, error) {
	_, err := s.InsertNotificationsForAllAssets(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to insert notifications")
		return nil, status.Error(codes.Internal, "Failed to insert notifications: "+err.Error())
	}

	return &assetpb.InsertAllResponse{
		Message: "Notifications successfully inserted",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *NotificationService) GetNotification(ctx context.Context, req *assetpb.GetNotificationsRequest) (*assetpb.GetNotificationsResponse, error) {
	log.Info().Msgf("Fetching notification with ID: %d", req.GetId())

	query := `
        SELECT n.id_notification, n.asset_id, n.submission_id, n.status, 
               a.asset_name, a.outlet_id, a.area_id, a.asset_maintenance_date
        FROM notifications n
        LEFT JOIN assets a ON a.asset_id = n.asset_id
        WHERE n.id_notification = $1
    `

	var notification assetpb.Notification

	err := s.DB.QueryRow(ctx, query, req.GetId()).Scan(
		&notification.IdNotification,
		&notification.AssetId,
		&notification.SubmissionId,
		&notification.Status,
		&notification.AssetName,
		&notification.OutletId,
		&notification.AreaId,
		&notification.MaintenanceOrSubmitted,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn().Msgf("Notification with ID %d not found", req.GetId())
			return nil, status.Error(codes.NotFound, "Notification not found")
		}
		log.Error().Err(err).Msg("Error fetching notification")
		return nil, status.Error(codes.Internal, "Failed to get notification")
	}

	return &assetpb.GetNotificationsResponse{
		Data:    &notification,
		Code:    "200",
		Message: "Successfully fetched notification by ID",
	}, nil
}

func (s *NotificationService) GetListNotification(ctx context.Context, req *assetpb.GetListNotificationRequest) (*assetpb.GetListNotificationResponse, error) {
	log.Info().Msg("Getting list of notifications")

	// Getting parameters from request
	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	outletId := req.OutletId
	areaId := req.AreaId
	roleId := req.RoleId

	if pageNumber <= 0 {
		pageNumber = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	offset := (pageNumber - 1) * pageSize

	// Query untuk mendapatkan daftar notifikasi dengan filter
	query := `
        SELECT id_notification, asset_id, submission_id, status, 
               asset_name, outlet_id, area_id, maintenance_or_submitted
        FROM notifications
        WHERE 1=1
    `
	var params []interface{}
	paramIndex := 1

	if q != "" {
		query += fmt.Sprintf(" AND asset_name LIKE $%d", paramIndex)
		params = append(params, "%"+q+"%")
		paramIndex++
	}

	if roleId == 5 {
		query += fmt.Sprintf(" AND area_id = $%d", paramIndex)
		params = append(params, areaId)
		paramIndex++
	} else if roleId == 6 {
		query += fmt.Sprintf(" AND outlet_id = $%d", paramIndex)
		params = append(params, outletId)
		paramIndex++
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
	params = append(params, pageSize, offset)

	rows, err := s.DB.Query(ctx, query, params...)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching notifications")
		return nil, err
	}
	defer rows.Close()

	var notifications []*assetpb.Notification
	for rows.Next() {
		var notif assetpb.Notification
		var submissionId sql.NullInt32
		var maintenanceOrSubmitted sql.NullTime

		err := rows.Scan(
			&notif.IdNotification,
			&notif.AssetId,
			&submissionId,
			&notif.Status,
			&notif.AssetName,
			&notif.OutletId,
			&notif.AreaId,
			&maintenanceOrSubmitted,
		)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}

		if maintenanceOrSubmitted.Valid {
			notif.MaintenanceOrSubmitted = maintenanceOrSubmitted.Time.Format("2006-01-02")
		} else {
			notif.MaintenanceOrSubmitted = ""
		}

		notifications = append(notifications, &notif)
	}

	// Query untuk mendapatkan total count berdasarkan status
	totalWaiting, _ := s.GetTotalCountWithFilters("notifications", q, outletId, areaId, "waiting", roleId)
	totalLate, _ := s.GetTotalCountWithFilters("notifications", q, outletId, areaId, "late", roleId)
	totalSubmitted, _ := s.GetTotalCountWithFilters("notifications", q, outletId, areaId, "submitted", roleId)

	totalCount := totalWaiting + totalLate + totalSubmitted

	resp := &assetpb.GetListNotificationResponse{
		Data:           notifications,
		TotalCount:     int32(totalCount),
		TotalWaiting:   int32(totalWaiting),
		TotalLate:      int32(totalLate),
		TotalSubmitted: int32(totalSubmitted),
		PageNumber:     pageNumber,
		PageSize:       pageSize,
	}

	// Menambahkan token halaman berikutnya jika masih ada data
	if int32(totalCount) > offset+pageSize {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	return resp, nil
}
func (s *NotificationService) GetTotalCountWithFilters(tableName string, q string, outletId, areaId int32, status string, roleId int32) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = $1", tableName)
	var params []interface{}
	params = append(params, status)
	paramIndex := 2

	if q != "" {
		query += fmt.Sprintf(" AND asset_name LIKE $%d", paramIndex)
		params = append(params, "%"+q+"%")
		paramIndex++
	}
	if roleId == 5 && areaId != 0 {
		query += fmt.Sprintf(" AND area_id = $%d", paramIndex)
		params = append(params, areaId)
		paramIndex++
	} else if roleId == 6 && outletId != 0 {
		query += fmt.Sprintf(" AND outlet_id = $%d", paramIndex)
		params = append(params, outletId)
		paramIndex++
	}

	var count int
	err := s.DB.QueryRow(context.Background(), query, params...).Scan(&count)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching total count")
		return 0, err
	}

	return count, nil
}
func (s *NotificationService) GetNotificationsFromDBWithFilters(offset, limit int, q string, outletId, areaId int32) ([]*assetpb.Notification, error) {
	log.Info().Msg("Fetching notifications from database with filters")

	query := `
        SELECT n.id_notification, n.asset_id, n.submission_id, n.status, 
               n.asset_name, n.outlet_id, n.area_id, n.maintenance_or_submitted,
               a.asset_maintenance_date
        FROM notifications n
        LEFT JOIN assets a ON a.asset_id = n.asset_id
        WHERE 1=1
    `
	var params []interface{}
	paramIndex := 1

	if q != "" {
		query += fmt.Sprintf(" AND n.asset_name LIKE $%d", paramIndex)
		params = append(params, "%"+q+"%")
		paramIndex++
	}

	if outletId != 0 {
		query += fmt.Sprintf(" AND n.outlet_id = $%d", paramIndex)
		params = append(params, outletId)
		paramIndex++
	}

	if areaId != 0 {
		query += fmt.Sprintf(" AND n.area_id = $%d", paramIndex)
		params = append(params, areaId)
		paramIndex++
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
	params = append(params, limit, offset)

	log.Info().Msgf("Executing query: %s with params: %v", query, params)

	rows, err := s.DB.Query(context.Background(), query, params...)
	if err != nil {
		log.Error().Err(err).Msg("Error executing query")
		return nil, err
	}
	defer rows.Close()

	var notifications []*assetpb.Notification
	for rows.Next() {
		var notif assetpb.Notification
		err := rows.Scan(
			&notif.IdNotification,
			&notif.AssetId,
			&notif.SubmissionId,
			&notif.Status,
			&notif.AssetName,
			&notif.OutletId,
			&notif.AreaId,
			&notif.MaintenanceOrSubmitted,
		)
		if err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}
		notifications = append(notifications, &notif)
	}

	log.Info().Msgf("Fetched %d notifications", len(notifications))
	return notifications, nil
}

func (s *NotificationService) GetNotificationById(id int32) (*assetpb.Notification, error) {
	log.Info().Msgf("Fetching notification with ID: %d", id)

	query := `
        SELECT n.id_notification, n.asset_id, n.submission_id, n.status, 
               n.asset_name, n.outlet_id, n.area_id, n.maintenance_or_submitted,
               a.asset_maintenance_date
        FROM notifications n
        LEFT JOIN assets a ON a.asset_id = n.asset_id
        WHERE n.id_notification = $1
    `

	log.Info().Msgf("Executing query: %s with param: %d", query, id)

	row := s.DB.QueryRow(context.Background(), query, id)

	var notification assetpb.Notification
	err := row.Scan(
		&notification.IdNotification,
		&notification.AssetId,
		&notification.SubmissionId,
		&notification.Status,
		&notification.AssetName,
		&notification.OutletId,
		&notification.AreaId,
		&notification.MaintenanceOrSubmitted,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Error().Msgf("Notification with ID %d not found", id)
			return nil, status.Error(codes.NotFound, "Notification not found")
		}
		log.Error().Err(err).Msg("Error executing query")
		return nil, status.Error(codes.Internal, "Failed to get notification")
	}

	log.Info().Msgf("Fetched notification: %+v", notification)
	return &notification, nil
}
