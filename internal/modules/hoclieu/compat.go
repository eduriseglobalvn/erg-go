package hoclieu

import (
	hoclieucontroller "erg.ninja/internal/modules/hoclieu/api/controller"
	hoclieudto "erg.ninja/internal/modules/hoclieu/api/dto"
	hoclieuservice "erg.ninja/internal/modules/hoclieu/application/service"
	hoclieurepository "erg.ninja/internal/modules/hoclieu/infrastructure/repository"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/storage"
)

type (
	Service                        = hoclieuservice.Service
	Repository                     = hoclieurepository.Repository
	Controller                     = hoclieucontroller.Controller
	ListResourceParams             = hoclieuservice.ListResourceParams
	AssetRecord                    = hoclieuservice.AssetRecord
	AssetFileType                  = hoclieudto.AssetFileType
	FileTypeWarningDTO             = hoclieudto.FileTypeWarningDTO
	FileTypeAuditDTO               = hoclieudto.FileTypeAuditDTO
	LaunchMode                     = hoclieudto.LaunchMode
	ProgramDTO                     = hoclieudto.ProgramDTO
	TaxonomyOptionDTO              = hoclieudto.TaxonomyOptionDTO
	TaxonomyResponseDTO            = hoclieudto.TaxonomyResponseDTO
	LectureDesignerPresetDTO       = hoclieudto.LectureDesignerPresetDTO
	HomeMetricDTO                  = hoclieudto.HomeMetricDTO
	HomeResponseDTO                = hoclieudto.HomeResponseDTO
	HomeHeroDTO                    = hoclieudto.HomeHeroDTO
	HomeShortcutDTO                = hoclieudto.HomeShortcutDTO
	ResourceCardDTO                = hoclieudto.ResourceCardDTO
	ResourceDetailDTO              = hoclieudto.ResourceDetailDTO
	ResourceItemDTO                = hoclieudto.ResourceItemDTO
	AssetDTO                       = hoclieudto.AssetDTO
	LectureDesignDTO               = hoclieudto.LectureDesignDTO
	LaunchResponseDTO              = hoclieudto.LaunchResponseDTO
	WatermarkDTO                   = hoclieudto.WatermarkDTO
	AuditMetadataDTO               = hoclieudto.AuditMetadataDTO
	ViewerPagesResponseDTO         = hoclieudto.ViewerPagesResponseDTO
	ViewerPageDTO                  = hoclieudto.ViewerPageDTO
	CreateResourceRequestDTO       = hoclieudto.CreateResourceRequestDTO
	UpdateResourceRequestDTO       = hoclieudto.UpdateResourceRequestDTO
	CreateAssetRequestDTO          = hoclieudto.CreateAssetRequestDTO
	UpdateAssetRequestDTO          = hoclieudto.UpdateAssetRequestDTO
	CreateTaxonomyOptionRequestDTO = hoclieudto.CreateTaxonomyOptionRequestDTO
	UpdateTaxonomyOptionRequestDTO = hoclieudto.UpdateTaxonomyOptionRequestDTO
	UploadAssetRequestDTO          = hoclieudto.UploadAssetRequestDTO
	UploadResourceResponseDTO      = hoclieudto.UploadResourceResponseDTO
)

const (
	AssetFileTypePDF   = hoclieudto.AssetFileTypePDF
	AssetFileTypePPTX  = hoclieudto.AssetFileTypePPTX
	AssetFileTypeVideo = hoclieudto.AssetFileTypeVideo
	AssetFileTypeAudio = hoclieudto.AssetFileTypeAudio
	AssetFileTypeHTML5 = hoclieudto.AssetFileTypeHTML5
	AssetFileTypeLink  = hoclieudto.AssetFileTypeLink
	AssetFileTypeQuiz  = hoclieudto.AssetFileTypeQuiz
	AssetFileTypeZIP   = hoclieudto.AssetFileTypeZIP
	AssetFileTypeDOCX  = hoclieudto.AssetFileTypeDOCX
	AssetFileTypeXLSX  = hoclieudto.AssetFileTypeXLSX
	AssetFileTypeImage = hoclieudto.AssetFileTypeImage

	LaunchModePDFReader       = hoclieudto.LaunchModePDFReader
	LaunchModeEBookReader     = hoclieudto.LaunchModeEBookReader
	LaunchModeSlideImageProxy = hoclieudto.LaunchModeSlideImageProxy
	LaunchModeGoogleSlide     = hoclieudto.LaunchModeGoogleSlide
	LaunchModeVideoPlayer     = hoclieudto.LaunchModeVideoPlayer
	LaunchModeAudioPlayer     = hoclieudto.LaunchModeAudioPlayer
	LaunchModeHTML5Embed      = hoclieudto.LaunchModeHTML5Embed
	LaunchModeQuizRuntime     = hoclieudto.LaunchModeQuizRuntime
	LaunchModeDownloadOnly    = hoclieudto.LaunchModeDownloadOnly
	LaunchModeExternal        = hoclieudto.LaunchModeExternal
)

var (
	ErrInvalidFileType      = hoclieuservice.ErrInvalidFileType
	ErrInvalidAssetMetadata = hoclieuservice.ErrInvalidAssetMetadata
	ErrInvalidRequest       = hoclieuservice.ErrInvalidRequest
	ErrNotFound             = hoclieuservice.ErrNotFound
)

func NewService(r2 ...*storage.R2Client) *Service {
	return hoclieuservice.NewService(r2...)
}

func NewRepository(mongoClient *database.MongoClient) *Repository {
	return hoclieurepository.NewRepository(mongoClient)
}

func NewController(svc *Service) *Controller {
	return hoclieucontroller.NewController(svc)
}

func DefaultLaunchMode(fileType AssetFileType) LaunchMode {
	return hoclieudto.DefaultLaunchMode(fileType)
}

func seedService(s *Service) {
	hoclieuservice.SeedService(s)
}
