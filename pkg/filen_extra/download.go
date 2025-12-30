package filenextra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
)

func CreateDownloadReader(ctx context.Context, c *filen.Filen, uuid string) (io.ReadCloser, error) {
	finfo, err := getV3FileInfo(ctx, c, uuid)
	if err != nil {
		return nil, err
	}

	chunks := int(finfo.Size) / filen.ChunkSize
	if int(finfo.Size)%filen.ChunkSize != 0 {
		chunks++
	}

	var metadata types.FileMetadata
	decryptedMetadata, err := c.DecryptMeta(crypto.EncryptedString(finfo.Metadata))
	if err != nil {
		return nil, fmt.Errorf("ReadDirectory decrypting metadata: %v", err)
	}
	err = json.Unmarshal([]byte(decryptedMetadata), &metadata)
	if err != nil {
		return nil, fmt.Errorf("ReadDirectory unmarshalling metadata: %v", err)
	}

	encryptionKey, err := crypto.MakeEncryptionKeyFromUnknownStr(metadata.Key)
	if err != nil {
		return nil, fmt.Errorf("ReadDirectory creating encryption key: %v", err)
	}

	filenFile := &types.File{
		IncompleteFile: types.IncompleteFile{
			Name:          metadata.Name,
			UUID:          finfo.Uuid,
			MimeType:      metadata.MimeType,
			EncryptionKey: *encryptionKey,
		},
		Region:  finfo.Region,
		Bucket:  finfo.Bucket,
		Size:    int(finfo.Size),
		Chunks:  chunks,
		Version: crypto.FileEncryptionVersion(finfo.Version),
	}

	return c.GetDownloadReader(ctx, filenFile), nil
}

type V3FileInfoResponse struct {
	Uuid          string `json:"uuid"`
	Region        string `json:"region"`
	Bucket        string `json:"bucket"`
	NameEncrypted string `json:"nameEncrypted"`
	NameHashed    string `json:"nameHashed"`
	SizeEncrypted string `json:"sizeEncrypted"`
	MimeEncrypted string `json:"mimeEncrypted"`
	Metadata      string `json:"metadata"`
	Size          int64  `json:"size"`
	Parent        string `json:"parent"`
	Versioned     bool   `json:"versioned"`
	Trash         bool   `json:"trash"`
	Version       int    `json:"version"`
}

func getV3FileInfo(ctx context.Context, c *filen.Filen, uuid string) (*V3FileInfoResponse, error) {
	var res V3FileInfoResponse
	_, err := c.Client.RequestData(ctx, "POST", client.GatewayURL("/v3/file"), struct {
		UUID string `json:"uuid"`
	}{UUID: uuid}, &res)
	return &res, err
}
