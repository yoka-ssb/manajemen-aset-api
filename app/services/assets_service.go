package services

import (
	"asset-management-api/assetpb"
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

type AssetService struct {
	MasterService
	assetpb.UnimplementedASSETServiceServer
}

func NewAssetService(db *gorm.DB) *AssetService {
	return &AssetService{
		MasterService: MasterService{DB: db}}
}

func (s *AssetService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterASSETServiceServer(grpcServer, s)
}

func (s *AssetService) CreateItem(ctx context.Context, req *assetpb.CreateItemRequest) (*assetpb.CreateItemResponse, error) {
	
	item := assetpb.Asset{
		AsetName: req.Item.GetAsetName(),
		AsetMerk: req.Item.GetAsetMerk(),
		AsetSpesifikasi: req.Item.GetAsetSpesifikasi(),
		AsetKlasifikasi: req.Item.GetAsetKlasifikasi(),
		AsetKondisi: req.Item.GetAsetKondisi(),
		AsetPic: req.Item.GetAsetPic(),
		AsetTglPembelian: req.Item.GetAsetTglPembelian(),
		AsetStatus: req.Item.GetAsetStatus(),
		KlasifikasiNilaiPerolehan: req.Item.GetKlasifikasiNilaiPerolehan(),
		KlasifikasiNilaiBukuTerakhir: req.Item.GetKlasifikasiNilaiBukuTerakhir(),
		AsetImage: req.Item.GetAsetImage(),
		NilaiPenyusutan: req.Item.GetNilaiPenyusutan(),
		Penanggungjawab: req.Item.GetPenanggungjawab(),
		OutletId: req.Item.GetOutletId(),
	}
	result := db.Create(&item)
	if result.Error != nil {
		log.Fatal(result.Error)
	}
	fmt.Printf("New item ID: %d\n", item.AsetId)

	return &assetpb.CreateItemResponse{Item: &assetpb.Asset{
		AsetId: item.AsetId,
		AsetName: item.AsetName,
		AsetMerk: item.AsetMerk,
		AsetSpesifikasi: item.AsetSpesifikasi,
		AsetKlasifikasi: item.AsetKlasifikasi,
		AsetKondisi: item.AsetKondisi,
		AsetPic: item.AsetPic,
		AsetTglPembelian: item.AsetTglPembelian,
		AsetStatus: item.AsetStatus,
		KlasifikasiNilaiPerolehan: item.KlasifikasiNilaiPerolehan,
		KlasifikasiNilaiBukuTerakhir: item.KlasifikasiNilaiBukuTerakhir,
		AsetImage: item.AsetImage,
		NilaiPenyusutan: item.NilaiPenyusutan,
		Penanggungjawab: item.Penanggungjawab,
		OutletId: item.OutletId,
	}}, nil
}

func (s *AssetService) GetItem(ctx context.Context, req *assetpb.GetItemRequest) (*assetpb.GetItemResponse, error) {
	log.Default().Println("getting item with ID: ", req.GetId())
	var item assetpb.Asset
	result := db.Where("aset_id = ?", req.GetId()).First(&item)
	if result.Error != nil {
		log.Println("Error:", result.Error)
	}
	return &assetpb.GetItemResponse{Item: &item, Success: true}, nil
}

func (s *AssetService) UpdateItem(ctx context.Context, req *assetpb.UpdateItemRequest) (*assetpb.UpdateItemResponse, error) {
	log.Default().Println("updating item")

	updates := map[string]interface{}{
		"AsetName": req.Item.GetAsetName(),
		"AsetMerk": req.Item.GetAsetMerk(),
		"AsetSpesifikasi": req.Item.GetAsetSpesifikasi(),
		"AsetKlasifikasi": req.Item.GetAsetKlasifikasi(),
		"AsetKondisi": req.Item.GetAsetKondisi(),
		"AsetPic": req.Item.GetAsetPic(),
		"AsetTglPembelian": req.Item.GetAsetTglPembelian(),
		"AsetStatus": req.Item.GetAsetStatus(),
		"KlasifikasiNilaiPerolehan": req.Item.GetKlasifikasiNilaiPerolehan(),
		"KlasifikasiNilaiBukuTerakhir": req.Item.GetKlasifikasiNilaiBukuTerakhir(),
		"AsetImage": req.Item.GetAsetImage(),
		"NilaiPenyusutan": req.Item.GetNilaiPenyusutan(),
		"Penanggungjawab": req.Item.GetPenanggungjawab(),
		"OutletId": req.Item.GetOutletId(),
	}
	result := db.Model(&assetpb.Asset{}).Where("aset_id = ?", req.Item.AsetId).Updates(updates)
	if result.Error != nil {
		log.Println("Error updating product:", result.Error)
	}

	return &assetpb.UpdateItemResponse{Item: &assetpb.Asset{
		AsetId: req.Item.GetAsetId(),
		AsetName: req.Item.GetAsetName(),
		AsetMerk: req.Item.GetAsetMerk(),
		AsetSpesifikasi: req.Item.GetAsetSpesifikasi(),
		AsetKlasifikasi: req.Item.GetAsetKlasifikasi(),
		AsetKondisi: req.Item.GetAsetKondisi(),
		AsetPic: req.Item.GetAsetPic(),
		AsetTglPembelian: req.Item.GetAsetTglPembelian(),
		AsetStatus: req.Item.GetAsetStatus(),
		KlasifikasiNilaiPerolehan: req.Item.GetKlasifikasiNilaiPerolehan(),
		KlasifikasiNilaiBukuTerakhir: req.Item.GetKlasifikasiNilaiBukuTerakhir(),
		AsetImage: req.Item.GetAsetImage(),
		NilaiPenyusutan: req.Item.GetNilaiPenyusutan(),
		Penanggungjawab: req.Item.GetPenanggungjawab(),
		OutletId: req.Item.GetOutletId(),
	}}, nil
}

func (s *AssetService) DeleteItem(ctx context.Context, req *assetpb.DeleteItemRequest) (*assetpb.DeleteItemResponse, error) {
	log.Default().Println("deleting item with ID: ", req.GetId())

	result := db.Delete(&assetpb.Asset{}, req.GetId())
	if result.Error != nil {
		log.Println("Error deleting product:", result.Error)
	}
	return &assetpb.DeleteItemResponse{Success: true}, nil
}

func (s *AssetService) ListItems(ctx context.Context, req *assetpb.ListItemsRequest) (*assetpb.ListItemsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("Failed to extract metadata from incoming context")
	}

	// Optional: Check for specific metadata key
	serverVersion := md.Get("server-version")
	if len(serverVersion) > 0 {
		log.Println("Server version:", serverVersion[0])
	} else {
		log.Println("Server version metadata not found")
	}

	log.Default().Println("listing items")
	var items []*assetpb.Asset
	result := db.Find(&items)
	if result.Error != nil {
		fmt.Println("Error fetching products:", result.Error)
	}

	return &assetpb.ListItemsResponse{Items: items}, nil
}