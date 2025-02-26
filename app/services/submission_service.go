package services

import (
	"asset-management-api/assetpb"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SubmissionService struct {
	MasterService
	assetpb.UnimplementedSUBMISSIONServiceServer
	DB *pgxpool.Pool
}

type Submission struct {
	SubmissionId          int32  `json:"submission_id,omitempty"`
	SubmissionName        string `json:"submission_name,omitempty"`
	SubmissionOutlet      string `json:"submission_outlet,omitempty"`
	SubmissionArea        string `json:"submission_area,omitempty"`
	SubmissionDate        string `json:"submission_date,omitempty"`
	SubmissionCategory    string `json:"submission_category,omitempty"`
	SubmissionStatus      string `json:"submission_status,omitempty"`
	SubmissionPurpose     string `json:"submission_purpose,omitempty"`
	SubmissionQuantity    int32  `json:"submission_quantity,omitempty"`
	SubmissionAssetName   string `json:"submission_asset_name,omitempty"`
	SubmissionDescription string `json:"submission_description,omitempty"`
	Nip                   int32  `json:"nip,omitempty"`
	AssetId               int32  `json:"asset_id,omitempty"`
	Attachment            string `json:"attachment,omitempty"`
	SubmissionPrName      string `json:"submission_pr_name,omitempty"`
	SubmissionRoleName    string `json:"submission_role_name,omitempty"`
	OutletId              int32  `json:"outlet_id,omitempty"`
	AreaId                int32  `json:"area_id,omitempty"`
	SubmissionPrice       int32  `json:"submission_price,omitempty"`
	SubmissionParentId    *int32 `json:"submission_parent_id,omitempty"`
}

type SubmissionParents struct {
	SubmissionParentId int32  `json:"submission_parent_id,omitempty"`
	Nip                string `json:"nip,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	OutletId           int32  `json:"outlet_id,omitempty"`
	AreaId             int32  `json:"area_id,omitempty"`
	OutletName         string `json:"outlet_name,omitempty"`
	AreaName           string `json:"area_name,omitempty"`
}

func NewSubmissionService(db *pgxpool.Pool) *SubmissionService {
	return &SubmissionService{
		MasterService: MasterService{},
		DB:            db,
	}
}

func (s *SubmissionService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterSUBMISSIONServiceServer(grpcServer, s)
}

func (s *SubmissionService) CreateSubmission(ctx context.Context, req *assetpb.CreateSubmissionRequest) (*assetpb.CreateSubmissionResponse, error) {
	log.Info().Msg("Creating submission")

	query := "SELECT asset_status, asset_name, outlet_name, area_name, asset_pic_name FROM assets WHERE asset_id = $1"
	var asset Submission
	err := s.DB.QueryRow(ctx, query, req.AssetId).Scan(&asset.SubmissionStatus, &asset.SubmissionAssetName, &asset.SubmissionOutlet, &asset.SubmissionArea, &asset.SubmissionRoleName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		return nil, status.Error(codes.Internal, "Failed to get asset")
	}

	if asset.SubmissionStatus != "Baik" || asset.SubmissionAssetName != req.SubmissionAssetName ||
		(req.SubmissionOutlet != "" && asset.SubmissionOutlet != req.SubmissionOutlet) ||
		(req.SubmissionArea != "" && asset.SubmissionArea != req.SubmissionArea) ||
		asset.SubmissionRoleName != req.SubmissionRoleName {
		return nil, status.Error(codes.NotFound, "Asset or related details do not match")
	}

	var lastID int32
	err = s.DB.QueryRow(ctx, "SELECT COALESCE(MAX(submission_id), 0) FROM submissions").Scan(&lastID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get last submission: "+err.Error())
	}
	submissionDate := time.Now().Format("2006-01-02")

	insertQuery := `INSERT INTO submissions (submission_id, submission_name, submission_outlet, outlet_id, area_id, submission_area, submission_date, submission_category, submission_status, submission_purpose, submission_asset_name, submission_quantity, submission_description, nip, asset_id, submission_pr_name, submission_role_name, attachment, submission_price) 
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`
	_, err = s.DB.Exec(ctx, insertQuery, lastID+1, req.SubmissionName, req.SubmissionOutlet, req.OutletId, req.AreaId, req.SubmissionArea, submissionDate, req.SubmissionCategory, req.SubmissionStatus, req.SubmissionPurpose, req.SubmissionAssetName, req.SubmissionQuantity, req.SubmissionDescription, req.Nip, req.AssetId, req.SubmissionPrName, req.SubmissionRoleName, req.Attachment, req.SubmissionPrice)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission: "+err.Error())
	}

	log.Info().Msg("Submission created successfully")

	return &assetpb.CreateSubmissionResponse{
		Message: "Successfully created submission",
		Code:    "200",
		Success: true,
	}, nil
}

// func sendEmail(toEmail, subject, body string) error {
// 	godotenv.Load(".env")

// 	// Setup SMTP server
// 	smtpHost := os.Getenv("SMTP_HOST")
// 	smtpPort := os.Getenv("SMTP_PORT")
// 	senderEmail := os.Getenv("SENDER_EMAIL")
// 	senderPassword := os.Getenv("SENDER_PASSWORD")

// 	msg := fmt.Sprintf(
// 		"From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
// 		senderEmail, toEmail, subject, body,
// 	)

// 	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

// 	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, senderEmail, []string{toEmail}, []byte(msg))
// 	if err != nil {
// 		return fmt.Errorf("failed to send email: %v", err)
// 	}
// 	return nil
// }

// // Persiapan email
// subject := "Pemberitahuan Pengajuan Maintenance Asset"
// body := fmt.Sprintf("Halo,\n\nPengajuan maintenance asset telah berhasil diajukan.\n\nDetail Pengajuan:\nAsset: %s\nKategori: %s\nStatus: %s\nTanggal Pengajuan: %s\n\nTerima kasih.",
// 	req.SubmissionAssetName, req.SubmissionCategory, req.SubmissionStatus, submissionDate)

// // Kirim email ke user yang memenuhi syarat
// for _, user := range users {
// 	if user.UserEmail != "" {
// 		if err := sendEmail(user.UserEmail, subject, body); err != nil {
// 			log.Println("Failed to send email to", user.UserEmail, err)
// 		} else {
// 			log.Println("Email sent to", user.UserEmail)
// 		}
// 	}
// }

func (s *SubmissionService) UpdateSubmissionStatus(ctx context.Context, req *assetpb.UpdateSubmissionStatusRequest) (*assetpb.UpdateSubmissionStatusResponse, error) {
	log.Info().Msgf("Updating submission status for ID: %d", req.Id)

	updateQuery := "UPDATE submissions SET submission_status = ? WHERE submission_id = ?"
	_, err := s.DB.Exec(ctx, updateQuery, req.Status, req.Id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update submission status")
		return nil, status.Error(codes.Internal, "Failed to update submission status: "+err.Error())
	}

	submissionQuery := "SELECT submission_name, submission_pr_name, asset_id FROM submissions WHERE submission_id = ?"
	var submissionName, submissionPrName string
	var assetId int32
	err = s.DB.QueryRow(ctx, submissionQuery, req.Id).Scan(&submissionName, &submissionPrName, &assetId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get submission")
		return nil, status.Error(codes.Internal, "Failed to get submission: "+err.Error())
	}

	logQuery := "INSERT INTO submission_logs (submission_id, status, description, pr_name) VALUES (?, ?, ?, ?)"
	_, err = s.DB.Exec(ctx, logQuery, req.Id, req.Status, "Status updated by "+submissionName, submissionPrName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create submission log")
		return nil, status.Error(codes.Internal, "Failed to create submission log: "+err.Error())
	}

	updateAssetQuery := "UPDATE assets SET asset_status = ? WHERE asset_id = ?"
	_, err = s.DB.Exec(ctx, updateAssetQuery, req.Status, assetId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update asset status")
		return nil, status.Error(codes.Internal, "Failed to update asset status: "+err.Error())
	}

	recordAssetUpdateQuery := "INSERT INTO asset_updates (asset_id, asset_status) VALUES (?, ?)"
	_, err = s.DB.Exec(ctx, recordAssetUpdateQuery, assetId, req.Status)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create asset update")
		return nil, status.Error(codes.Internal, "Failed to create asset update: "+err.Error())
	}

	log.Info().Msg("Successfully updated submission status")

	return &assetpb.UpdateSubmissionStatusResponse{
		Message: "Successfully updated submission status",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *SubmissionService) ListSubmissions(ctx context.Context, req *assetpb.ListSubmissionsRequest) (*assetpb.ListSubmissionsResponse, error) {
	log.Info().Msg("Listing submissions")

	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	roleID := req.GetRoleId()
	areaID := req.GetAreaId()
	outletID := req.GetOutletId()
	submissionParentID := req.GetSubmissionParentId()
	parentID := req.GetParentId()

	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	submissionQuery := `SELECT id, name, status, created_at FROM submissions 
		WHERE name LIKE ? AND role_id = ? AND area_id = ? AND outlet_id = ? AND submission_parent_id = ? AND parent_id = ? 
		LIMIT ? OFFSET ?`

	rows, err := s.DB.Query(ctx, submissionQuery, "%"+q+"%", roleID, areaID, outletID, submissionParentID, parentID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching submissions")
		return nil, err
	}
	defer rows.Close()

	var submissions []*assetpb.Submission
	for rows.Next() {
		var submission assetpb.Submission
		if err := rows.Scan(&submission.SubmissionId, &submission.SubmissionName, &submission.SubmissionDate); err != nil {
			log.Error().Err(err).Msg("Error scanning submission row")
			return nil, err
		}
		submissions = append(submissions, &submission)
	}

	totalCountQuery := `SELECT COUNT(*) FROM submissions WHERE name LIKE ? AND role_id = ? AND area_id = ? AND outlet_id = ? AND submission_parent_id = ? AND parent_id = ?`
	var totalCount int32
	err = s.DB.QueryRow(ctx, totalCountQuery, "%"+q+"%", roleID, areaID, outletID, submissionParentID, parentID).Scan(&totalCount)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching total count")
		return nil, err
	}

	resp := &assetpb.ListSubmissionsResponse{
		Data:       submissions,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("%d", pageNumber+1)
	} else {
		resp.NextPageToken = ""
	}

	return resp, nil
}

func GetSubmissionTotalCount(db *pgxpool.Pool, q string, roleID int32, areaID int32, outletID int32, submissionParentID int32, parentID bool) (int32, error) {
	query := "SELECT COUNT(*) FROM submissions WHERE 1=1"
	var args []interface{}

	if q != "" {
		query += " AND submission_name LIKE $1"
		args = append(args, "%"+q+"%")
	}
	if roleID == 5 && areaID != 0 {
		query += " AND area_id = $2"
		args = append(args, areaID)
	}
	if roleID == 6 && outletID != 0 {
		query += " AND outlet_id = $3"
		args = append(args, outletID)
	}
	if submissionParentID != 0 {
		query += " AND submission_parent_id = $4"
		args = append(args, submissionParentID)
	}
	if parentID {
		query += " AND submission_parent_id IS NOT NULL"
	} else {
		query += " AND submission_parent_id IS NULL"
	}

	var count int32
	err := db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetTotalCountByCategory(db *pgxpool.Pool, category string) (int32, error) {
	query := "SELECT COUNT(*) FROM submissions WHERE submission_category = $1"
	var count int32
	err := db.QueryRow(context.Background(), query, category).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetSubmissions(db *pgxpool.Pool, offset, limit int32, q string, roleID int32, areaID int32, outletID int32, submissionParentID int32, parentID bool) ([]*assetpb.Submission, error) {
	query := "SELECT submissions.*, assets.asset_id FROM submissions LEFT JOIN assets ON assets.asset_id = submissions.asset_id WHERE 1=1"
	var args []interface{}

	if q != "" {
		query += " AND submissions.submission_name LIKE $1"
		args = append(args, "%"+q+"%")
	}
	if roleID == 5 && areaID != 0 {
		query += " AND submissions.area_id = $2"
		args = append(args, areaID)
	}
	if roleID == 6 && outletID != 0 {
		query += " AND submissions.outlet_id = $3"
		args = append(args, outletID)
	}
	if submissionParentID != 0 {
		query += " AND submissions.submission_parent_id = $4"
		args = append(args, submissionParentID)
	}
	if parentID {
		query += " AND submissions.submission_parent_id IS NOT NULL"
	} else {
		query += " AND submissions.submission_parent_id IS NULL"
	}

	query += " ORDER BY submissions.submission_date ASC LIMIT $5 OFFSET $6"
	args = append(args, limit, offset)

	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Error executing query")
		return nil, err
	}
	defer rows.Close()

	var submissions []*assetpb.Submission
	for rows.Next() {
		var submission assetpb.Submission
		if err := rows.Scan(&submission); err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}
		submissions = append(submissions, &submission)
	}

	log.Info().Msgf("Query executed successfully, found %d submissions", len(submissions))

	return submissions, nil
}

func (s *SubmissionService) GetRoleIDByNIP(nip string) (int32, error) {
	query := "SELECT role_id FROM users WHERE nip = $1"
	var roleID int32
	err := s.DB.QueryRow(context.Background(), query, nip).Scan(&roleID)
	if err != nil {
		return 0, err
	}
	return roleID, nil
}

func GetSubmissionById(db *pgxpool.Pool, id int32) (*assetpb.Submission, error) {
	query := "SELECT submissions.*, assets.asset_id, assets.outlet_id, assets.area_id FROM submissions LEFT JOIN assets ON assets.asset_id = submissions.asset_id WHERE submissions.submission_id = $1"
	var submission assetpb.Submission
	log.Info().Msgf("Fetching submission with ID: %d", id)
	err := db.QueryRow(context.Background(), query, id).Scan(&submission)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching submission")
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "Submission not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get submission")
		}
	}
	return &submission, nil
}

func (s *SubmissionService) GetSubmissionById(ctx context.Context, req *assetpb.GetSubmissionByIdRequest) (*assetpb.GetSubmissionByIdResponse, error) {
	log.Info().Msgf("Fetching submission with ID: %d", req.Id)

	submission, err := GetSubmissionById(s.DB, req.Id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get submission")
		return nil, err
	}

	return &assetpb.GetSubmissionByIdResponse{
		Submission: submission,
	}, nil
}

func (s *SubmissionService) CreateSubmissionParent(ctx context.Context, req *assetpb.CreateSubmissionParentRequest) (*assetpb.CreateSubmissionParentResponse, error) {
	log.Info().Msg("Creating submission parent")

	var lastSubmissionParentId int32
	lastQuery := "SELECT COALESCE(MAX(submission_parent_id), 0) FROM submission_parents"
	err := s.DB.QueryRow(ctx, lastQuery).Scan(&lastSubmissionParentId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get last submission parent")
		return nil, status.Error(codes.Internal, "Failed to get last submission parent: "+err.Error())
	}

	newSubmissionParentId := lastSubmissionParentId + 1

	insertQuery := "INSERT INTO submission_parents (submission_parent_id, nip, created_at, outlet_id, area_id) VALUES ($1, $2, $3, $4, $5)"
	_, err = s.DB.Exec(ctx, insertQuery, newSubmissionParentId, req.Nip, time.Now().Format("2006-01-02 15:04:05"), req.OutletId, req.AreaId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create submission parent")
		return nil, status.Error(codes.Internal, "Failed to create submission parent: "+err.Error())
	}

	updateQuery := "UPDATE submissions SET submission_parent_id = $1 WHERE submission_id = $2"
	for _, submissionId := range req.SubmissionIds {
		_, err = s.DB.Exec(ctx, updateQuery, newSubmissionParentId, submissionId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to update submission with parent ID")
			return nil, status.Error(codes.Internal, "Failed to update submissions: "+err.Error())
		}
	}

	return &assetpb.CreateSubmissionParentResponse{
		Message:            "Successfully created submission parent",
		Code:               "200",
		Success:            true,
		SubmissionParentId: newSubmissionParentId,
	}, nil
}

func (s *SubmissionService) CreateSubmissionParentHandler(c *gin.Context) {
	var req assetpb.CreateSubmissionParentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.CreateSubmissionParent(context.Background(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *SubmissionService) updateSubmissionsWithParentID(submissionParentId int32, submissionIds []int32) error {
	log.Info().Msgf("Updating submissions with submission_parent_id: %d", submissionParentId)

	query := "UPDATE submissions SET submission_parent_id = $1 WHERE submission_id = ANY($2)"
	args := []interface{}{submissionParentId, submissionIds}

	res, err := s.DB.Exec(context.Background(), query, args...)
	if err != nil {
		return fmt.Errorf("failed to update submissions with new submission_parent_id: %v", err)
	}

	rowsAffected := res.RowsAffected()
	if rowsAffected == 0 {
		log.Warn().Msg("No rows were updated. Check if submission IDs exist.")
	}

	return nil
}

func GetSubmissionParents(db *pgxpool.Pool, offset, limit int32, q, nip string, roleID int, areaID, outletID int) ([]*SubmissionParents, error) {
	query := `SELECT sp.submission_parent_id, sp.nip, sp.created_at, o.outlet_name, a.area_name 
            FROM submission_parents sp
            LEFT JOIN outlets o ON o.outlet_id = sp.outlet_id
            LEFT JOIN areas a ON a.area_id = sp.area_id
            WHERE 1=1`

	var args []interface{}
	argIndex := 1

	if q != "" {
		query += fmt.Sprintf(" AND sp.nip LIKE $%d", argIndex)
		args = append(args, "%"+q+"%")
		argIndex++
	}
	if nip != "" {
		query += fmt.Sprintf(" AND sp.nip = $%d", argIndex)
		args = append(args, nip)
		argIndex++
	}

	switch roleID {
	case 5:
		query += fmt.Sprintf(" AND sp.area_id = $%d", argIndex)
		args = append(args, areaID)
		argIndex++
	case 6:
		query += fmt.Sprintf(" AND sp.outlet_id = $%d", argIndex)
		args = append(args, outletID)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY sp.submission_parent_id ASC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Error executing query")
		return nil, err
	}
	defer rows.Close()

	var submissionParents []*SubmissionParents
	for rows.Next() {
		var sp SubmissionParents
		if err := rows.Scan(&sp.SubmissionParentId, &sp.Nip, &sp.CreatedAt, &sp.OutletName, &sp.AreaName); err != nil {
			log.Error().Err(err).Msg("Error scanning row")
			return nil, err
		}

		createdAt, err := time.Parse("2006-01-02 15:04:05", sp.CreatedAt)
		if err == nil {
			sp.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		}

		submissionParents = append(submissionParents, &sp)
	}

	log.Info().Msgf("Query executed successfully, found %d submission parents", len(submissionParents))
	return submissionParents, nil
}

func (s *SubmissionService) ListSubmissionParents(ctx context.Context, req *assetpb.ListSubmissionParentsRequest) (*assetpb.ListSubmissionParentsResponse, error) {
	log.Info().Msg("Listing submission parents")

	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	nip := req.GetNip()
	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	query := `SELECT submission_parents.submission_parent_id, submission_parents.nip, submission_parents.created_at, 
                outlets.outlet_name, areas.area_name 
             FROM submission_parents 
             LEFT JOIN outlets ON outlets.outlet_id = submission_parents.outlet_id 
             LEFT JOIN areas ON areas.area_id = submission_parents.area_id`

	var args []interface{}
	var conditions []string
	argIndex := 1

	if q != "" {
		conditions = append(conditions, fmt.Sprintf("submission_parents.nip LIKE $%d", argIndex))
		args = append(args, "%"+q+"%")
		argIndex++
	}
	if nip != "" {
		conditions = append(conditions, fmt.Sprintf("submission_parents.nip = $%d", argIndex))
		args = append(args, nip)
		argIndex++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY submission_parents.submission_parent_id ASC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching submission parents")
		return nil, err
	}
	defer rows.Close()

	var submissionParents []*assetpb.SubmissionParent
	for rows.Next() {
		var sp assetpb.SubmissionParent
		if err := rows.Scan(&sp.SubmissionParentId, &sp.Nip, &sp.CreatedAt, &sp.OutletName, &sp.AreaName); err != nil {
			log.Error().Err(err).Msg("Error scanning submission parent row")
			return nil, err
		}
		submissionParents = append(submissionParents, &sp)
	}

	totalQuery := "SELECT COUNT(*) FROM submission_parents"
	var totalCount int32
	err = s.DB.QueryRow(ctx, totalQuery).Scan(&totalCount)
	if err != nil {
		log.Error().Err(err).Msg("Error fetching total count")
		return nil, err
	}

	resp := &assetpb.ListSubmissionParentsResponse{
		Data:       submissionParents,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("%d", pageNumber+1)
	} else {
		resp.NextPageToken = ""
	}

	return resp, nil
}

func (s *SubmissionService) GetSubmissionParentsTotalCount(q string, nip string, roleID, areaID, outletID int) (int32, error) {
	var count int32
	query := "SELECT COUNT(*) FROM submission_parents WHERE 1=1"
	var args []interface{}
	argIndex := 1

	if q != "" {
		query += fmt.Sprintf(" AND nip LIKE $%d", argIndex)
		args = append(args, "%"+q+"%")
		argIndex++
	}
	if nip != "" {
		query += fmt.Sprintf(" AND nip = $%d", argIndex)
		args = append(args, nip)
		argIndex++
	}

	switch roleID {
	case 5:
		query += fmt.Sprintf(" AND area_id = $%d", argIndex)
		args = append(args, areaID)
		argIndex++
	case 6:
		query += fmt.Sprintf(" AND outlet_id = $%d", argIndex)
		args = append(args, outletID)
		argIndex++
	}

	err := s.DB.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SubmissionService) ListSubmissionParentsHandler(c *gin.Context) {
	pageNumberParam := c.DefaultQuery("page_number", "1")
	pageSizeParam := c.DefaultQuery("page_size", "10")
	q := c.DefaultQuery("q", "")
	nip := c.DefaultQuery("nip", "")

	pageNumber, err := strconv.Atoi(pageNumberParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_number"})
		return
	}

	pageSize, err := strconv.Atoi(pageSizeParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_size"})
		return
	}

	roleID, _ := strconv.Atoi(c.GetHeader("role_id"))
	areaID, _ := strconv.Atoi(c.GetHeader("area_id"))
	outletID, _ := strconv.Atoi(c.GetHeader("outlet_id"))

	req := &assetpb.ListSubmissionParentsRequest{
		PageNumber: int32(pageNumber),
		PageSize:   int32(pageSize),
		Q:          q,
		Nip:        nip,
	}

	log.Info().Msgf("ListSubmissionParentsHandler called with parameters - pageNumber: %d, pageSize: %d, q: %s, nip: %s, roleID: %d, areaID: %d, outletID: %d", pageNumber, pageSize, q, nip, roleID, areaID, outletID)

	resp, err := s.ListSubmissionParents(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
