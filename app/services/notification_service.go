package services

import (
    "asset-management-api/assetpb"
    "context"
    "errors"
    "fmt"
    "time"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "gorm.io/gorm"
)

type Notification struct {
	NotificationId         int32  `gorm:"column:id_notification;primaryKey;autoIncrement" json:"id_notification,omitempty"`
	AssetId                int32  `gorm:"column:asset_id" json:"asset_id,omitempty"`
	SubmissionId           *int32 `gorm:"column:submission_id" json:"submission_id,omitempty"`
	AssetName              string `gorm:"column:asset_name" json:"asset_name,omitempty"`
	OutletId               int32  `gorm:"column:outlet_id" json:"outlet_id,omitempty"`
	AreaId                 int32  `gorm:"column:area_id" json:"area_id,omitempty"`
	MaintenanceDate        string `gorm:"column:maintenance_or_submitted" json:"maintenance_or_submitted,omitempty"`
	NotificationStatus     string `gorm:"column:status" json:"status,omitempty"`
	MaintenanceOrSubmitted string `gorm:"column:maintenance_or_submitted" json:"maintenance_or_submitted,omitempty"`
}

func (s *NotificationService) GetTotalCountWithOutlet(tableName string, outletId int32) (int64, error) {
	var count int64
	err := s.DB.Table(tableName).Where("outlet_id = ?", outletId).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

type NotificationService struct {
	MasterService
	assetpb.UnimplementedNOTIFICATIONServiceServer
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{
		MasterService: MasterService{DB: db},
	}
}

func (s *NotificationService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterNOTIFICATIONServiceServer(grpcServer, s)
}

// func (s *NotificationService) InsertNotification(ctx context.Context, req *assetpb.InsertNotificationRequest) (*assetpb.InsertNotificationResponse, error) {
// 	log.Println("Inserting notification")

// 	log.Printf("MaintenanceDate string received: %s", req.MaintenanceOrSubmitted)

// 	maintenanceDate, err := time.Parse("2006-01-02", req.MaintenanceOrSubmitted)
// 	if err != nil {
// 		log.Printf("Failed to parse maintenance date: %s", err)
// 		return nil, status.Error(codes.InvalidArgument, "Invalid date format: "+err.Error())
// 	}

// 	if maintenanceDate.Before(time.Now()) {

// 		var existingNotification Notification
// 		if err := s.DB.Where("asset_id = ? AND asset_maintenance_date = ?", req.AssetId, req.MaintenanceOrSubmitted).First(&existingNotification).Error; err == nil {

// 			return nil, status.Error(codes.AlreadyExists, "Notification for this asset and maintenance date already exists")
// 		} else if !errors.Is(err, gorm.ErrRecordNotFound) {

// 			return nil, status.Error(codes.Internal, "Failed to check existing notification: "+err.Error())
// 		}

// 		var asset Asset
// 		if err := s.DB.Where("asset_id = ?", req.AssetId).First(&asset).Error; err != nil {
// 			if errors.Is(err, gorm.ErrRecordNotFound) {
// 				return nil, status.Error(codes.NotFound, "Asset not found")
// 			}
// 			return nil, status.Error(codes.Internal, "Failed to fetch asset data: "+err.Error())
// 		}

// 		var lastNotification Notification
// 		if err := s.DB.Order("id_notification desc").First(&lastNotification).Error; err != nil {
// 			if !errors.Is(err, gorm.ErrRecordNotFound) {
// 				return nil, status.Error(codes.Internal, "Failed to retrieve last notification: "+err.Error())
// 			}

// 			lastNotification.NotificationId = 0
// 		}

// 		notification := Notification{
// 			NotificationId:         lastNotification.NotificationId + 1,
// 			AssetId:                req.AssetId,
// 			AssetName:              asset.AssetName,
// 			OutletId:               asset.OutletId,
// 			AreaId:                 asset.AreaId,
// 			MaintenanceOrSubmitted: maintenanceDate.Format("2006-01-02"),
// 			NotificationStatus:     req.Status,
// 		}

// 		if err := s.DB.Create(&notification).Error; err != nil {
// 			log.Printf("Failed to insert notification for asset %d: %v", req.AssetId, err)
// 			return nil, status.Error(codes.Internal, "Failed to insert notification: "+err.Error())
// 		}
// 	} else {
// 		return nil, status.Error(codes.InvalidArgument, "Asset maintenance date has not passed yet")
// 	}

// 	return &assetpb.InsertNotificationResponse{
// 		Message: "Notification inserted successfully",
// 		Code:    "200",
// 		Success: true,
// 	}, nil
// }
func (s *NotificationService) InsertNotificationsForAllAssets(ctx context.Context, req *assetpb.InsertAllRequest) (*assetpb.InsertAllResponse, error) {
    log.Info().Msg("Processing notifications based on asset maintenance dates and submissions")

    // Step 1: Ambil data dari tabel assets
    var assets []Asset
    if err := s.DB.Find(&assets).Error; err != nil {
        log.Error().Err(err).Msg("Failed to retrieve assets")
        return nil, err
    }

    log.Info().Msgf("Found %d assets", len(assets))

    // Step 2: Proses data assets
    for _, asset := range assets {
        log.Info().Msgf("Processing asset ID: %d, Name: %s, Maintenance Date: %s, Outlet ID: %d, Area ID: %d", asset.AssetId, asset.AssetName, asset.AssetMaintenanceDate, asset.OutletId, asset.AreaId)

        // Parse tanggal maintenance
        maintenanceDate, err := time.Parse(time.RFC3339, asset.AssetMaintenanceDate)
        if err != nil {
            log.Error().Err(err).Msgf("Failed to parse maintenance date for asset ID %d", asset.AssetId)
            continue
        }

        var existingNotification Notification
        err = s.DB.Where("asset_id = ?", asset.AssetId).First(&existingNotification).Error

        // Hitung selisih hari antara tanggal maintenance dan sekarang
        daysUntilMaintenance := int(time.Until(maintenanceDate).Hours() / 24)

        var notificationStatus string
        if daysUntilMaintenance < 0 {
            notificationStatus = "late"
        } else if daysUntilMaintenance <= 7 {
            notificationStatus = "waiting"
        } else {
            notificationStatus = "normal" // Status baru untuk maintenance date yang masih jauh
        }

        // Tentukan maintenance_or_submitted
        maintenanceOrSubmitted := maintenanceDate.Format("2006-01-02")
        if notificationStatus == "normal" {
            // Hapus data dari tabel Notification jika statusnya normal
            if err == nil {
                log.Info().Msgf("Deleting notification for asset ID %d (status: normal)", asset.AssetId)
                if err = s.DB.Delete(&Notification{}, "asset_id = ?", asset.AssetId).Error; err != nil {
                    log.Error().Err(err).Msgf("Failed to delete notification for asset ID %d", asset.AssetId)
                }
            }
        } else {
            // Cek jika data sudah ada
            if err == nil {
                // Update jika data di tabel Notification sudah ada
                existingNotification.MaintenanceDate = maintenanceDate.Format("2006-01-02")
                existingNotification.NotificationStatus = notificationStatus
                existingNotification.MaintenanceOrSubmitted = maintenanceOrSubmitted
                if err = s.DB.Save(&existingNotification).Error; err != nil {
                    log.Error().Err(err).Msgf("Failed to update notification for asset ID %d", asset.AssetId)
                }
            } else if errors.Is(err, gorm.ErrRecordNotFound) {
                // Masukkan data baru ke tabel Notification
                log.Info().Msgf("Creating new notification for asset ID %d", asset.AssetId)
                notification := Notification{
                    AssetId:            asset.AssetId,
                    AssetName:          asset.AssetName,
                    OutletId:           asset.OutletId,
                    AreaId:             asset.AreaId,
                    MaintenanceDate:    maintenanceDate.Format("2006-01-02"),
                    NotificationStatus: notificationStatus,
                }
                log.Info().Msgf("New notification details: Asset ID: %d, Outlet ID: %d, Area ID: %d", notification.AssetId, notification.OutletId, notification.AreaId)
                if err = s.DB.Create(&notification).Error; err != nil {
                    log.Error().Err(err).Msgf("Failed to create notification for asset ID %d", asset.AssetId)
                }
            }
        }
    }

    // Step 3: Ambil data dari tabel submissions
    var submissions []Submission
    if err := s.DB.Find(&submissions).Error; err != nil {
        log.Error().Err(err).Msg("Failed to retrieve submissions")
        return nil, err
    }

    log.Info().Msgf("Found %d submissions", len(submissions))

    // Step 4: Proses data submissions
    for _, submission := range submissions {
        log.Info().Msgf("Processing submission ID: %d, Asset ID: %d, Outlet ID: %d, Area ID: %d", submission.SubmissionId, submission.AssetId, submission.OutletId, submission.AreaId)

        var existingNotification Notification
        err := s.DB.Where("asset_id = ? AND status = ?", submission.AssetId, "submitted").
            First(&existingNotification).Error

        if errors.Is(err, gorm.ErrRecordNotFound) {
            // Jika tidak ada, masukkan data baru ke tabel Notification
            log.Info().Msgf("Creating new notification for submission ID %d", submission.SubmissionId)
            notification := Notification{
                AssetId:            submission.AssetId,
                SubmissionId:       &submission.SubmissionId,
                AssetName:          submission.SubmissionAssetName,
                OutletId:           submission.OutletId,
                AreaId:             submission.AreaId,
                MaintenanceDate:    time.Now().Format("2006-01-02"),
                NotificationStatus: "submitted",
            }
            log.Info().Msgf("New notification details: Asset ID: %d, Outlet ID: %d, Area ID: %d", notification.AssetId, notification.OutletId, notification.AreaId)
            if err = s.DB.Create(&notification).Error; err != nil {
                log.Error().Err(err).Msgf("Failed to create notification for submission ID %d", submission.SubmissionId)
            }
        } else if err != nil {
            log.Error().Err(err).Msgf("Error while checking notification for submission ID %d", submission.SubmissionId)
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
		return nil, status.Error(codes.Internal, "Failed to insert notifications: "+err.Error())
	}

	return &assetpb.InsertAllResponse{}, nil
}
func (s *NotificationService) GetNotification(ctx context.Context, req *assetpb.GetNotificationsRequest) (*assetpb.GetNotificationsResponse, error) {
    log.Info().Msgf("Fetching notification with ID: %d", req.GetId())

    var notification assetpb.Notification

    query := s.DB.Select("notifications.*, assets.asset_id, assets.asset_name, assets.outlet_id, assets.area_id, assets.asset_maintenance_date").
        Joins("LEFT JOIN assets ON assets.asset_id = notifications.asset_id").
        Where("notifications.notification_id = ?", req.GetId())

    result := query.First(&notification)
    if result.Error != nil {
        log.Error().Err(result.Error).Msg("Error fetching notification")
        if errors.Is(result.Error, gorm.ErrRecordNotFound) {
            return nil, status.Error(codes.NotFound, "Notification not found")
        } else {
            return nil, status.Error(codes.Internal, "Failed to get notification")
        }
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

    var notifications []*assetpb.Notification
    var err error
    if roleId == 5 {
        notifications, err = s.GetNotificationsFromDBWithFilters(int(offset), int(pageSize), q, 0, areaId) // Only filter by areaId
    } else if roleId == 6 {
        notifications, err = s.GetNotificationsFromDBWithFilters(int(offset), int(pageSize), q, outletId, 0) // Only filter by outletId
    } else {
        notifications, err = s.GetNotificationsFromDBWithFilters(int(offset), int(pageSize), q, 0, 0) // No filters by outletId or areaId
    }

    if err != nil {
        log.Error().Err(err).Msg("Error fetching notifications")
        return nil, err
    }

    totalWaiting, err := s.GetTotalCountWithFilters("notifications", outletId, areaId, "waiting")
    if err != nil {
        log.Error().Err(err).Msg("Error fetching total count (waiting)")
        return nil, err
    }

    totalLate, err := s.GetTotalCountWithFilters("notifications", outletId, areaId, "late")
    if err != nil {
        log.Error().Err(err).Msg("Error fetching total count (late)")
        return nil, err
    }

    totalSubmitted, err := s.GetTotalCountWithFilters("notifications", outletId, areaId, "submitted")
    if err != nil {
        log.Error().Err(err).Msg("Error fetching total count (submitted)")
        return nil, err
    }

    resp := &assetpb.GetListNotificationResponse{
        Data:           notifications,
        TotalCount:     int32(totalWaiting + totalLate + totalSubmitted),
        TotalWaiting:   int32(totalWaiting),
        TotalLate:      int32(totalLate),
        TotalSubmitted: int32(totalSubmitted),
        PageNumber:     pageNumber,
        PageSize:       pageSize,
    }

    if int32(totalWaiting+totalLate+totalSubmitted) > offset+pageSize {
        resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
    }

    return resp, nil
}

func (s *NotificationService) GetTotalCountWithFilters(tableName string, outletId, areaId int32, status string) (int, error) {
    var count int64
    query := s.DB.Table(tableName)

    if outletId != 0 {
        query = query.Where("outlet_id = ?", outletId)
    }

    if areaId != 0 {
        query = query.Where("area_id = ?", areaId)
    }

    query = query.Where("status = ?", status)

    err := query.Count(&count).Error
    if err != nil {
        log.Error().Err(err).Msg("Error fetching total count with filters")
        return 0, err
    }

    return int(count), nil
}

func (s *NotificationService) GetNotificationsFromDBWithFilters(offset, limit int, q string, outletId, areaId int32) ([]*assetpb.Notification, error) {
	var notifications []*assetpb.Notification

	query := s.DB.Select("notifications.*, assets.asset_id, assets.asset_name, assets.outlet_id, assets.area_id, assets.asset_maintenance_date").
		Joins("LEFT JOIN assets ON assets.asset_id = notifications.asset_id").
		Where("notifications.asset_name LIKE ?", "%"+q+"%")

	if outletId != 0 {
		query = query.Where("notifications.outlet_id = ?", outletId)
	}

	if areaId != 0 {
		query = query.Where("notifications.area_id = ?", areaId)
	}

	query = query.Offset(offset).Limit(limit)

	result := query.Find(&notifications)
	if result.Error != nil {
		return nil, result.Error
	}

	return notifications, nil
}

func (s *NotificationService) GetNotificationById(id int32) (*assetpb.Notification, error) {
	var notification assetpb.Notification

	query := db.Select("notications.*, assets.asset_id AS asset_id, assets.asset_name AS asset_name, assets.outlet_id AS outlet_id, assets.area_id AS area_id, assets.asset_maintenance_date AS maintenance_or_submitted").
		Joins("LEFT JOIN assets ON assets.asset_name = notifications.asset_name").
		Joins("LEFT JOIN assets ON assets.outlet_id = notifications.outlet_id").
		Joins("LEFT JOIN assets ON assets.area_id = notifications.area_id").
		Joins("LEFT JOIN assets ON assets.asset_maintenance_date = notifications.maintenance_or_submitted").
		Where("notifications.notification_id = ?", id)

	result := query.First(&notification)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("Error fetching notification")
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "notification not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get notification")
		}
	}
	return &notification, nil
}
