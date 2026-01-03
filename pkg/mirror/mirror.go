package mirror

import (
	"context"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/executer"
	"github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/filedb"
	filenextra "github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/filen_extra"
	"github.com/rs/zerolog/log"
)

type FilenMirrorConfig struct {
	SyncDir string
}

type FilenMirror struct {
	client             *filen.Filen
	filenEventListener *filenextra.FilenEventListener
	osDb               *filedb.FileTree
	baseDirUuid        filedb.Uuid
	syncDir            string
	taskRunner         *TaskRunner
}

func NewFilenMirror(client *filen.Filen, events *filenextra.FilenEventListener, cfg FilenMirrorConfig) *FilenMirror {
	baseDirUuid := filedb.UuidFromString(client.BaseFolder.UUID)

	return &FilenMirror{
		filenEventListener: events,
		client:             client,
		osDb:               filedb.NewFileTree(),
		baseDirUuid:        baseDirUuid,
		syncDir:            cfg.SyncDir,
		taskRunner:         NewTaskRunner(),
	}
}

func (m *FilenMirror) fullSyncOnce(ctx context.Context) error {
	err := m.removeLocalDbItemsNotInFs(m.osDb.GetPathToUuidMap())
	if err != nil {
		return err
	}

	remoteDb, err := m.fetchRemoteDb(ctx)
	if err != nil {
		return err
	}

	diffChannel := filedb.StartDiff(m.osDb, remoteDb)
	m.applyDiffItems(diffChannel, remoteDb)

	m.osDb.CopyFrom(remoteDb)

	err = m.removeLocalFilesNotInDb(remoteDb.GetPathToUuidMap())
	if err != nil {
		return err
	}

	return nil
}

func (m *FilenMirror) removeLocalDbItemsNotInFs(paths map[string]filedb.Uuid) error {
	err := fastReadDirDirs(m.syncDir, func(p string, isDir bool, continueDescending *bool) {
		*continueDescending = true
		relPath := strings.TrimPrefix(p, m.syncDir+"/")
		delete(paths, relPath)
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, pathUuid := range paths {
		log.Info().Msgf("Removing local DB item not in FS: %s", pathUuid)
		m.osDb.Remove(pathUuid)
	}

	return nil
}

func (m *FilenMirror) removeLocalFilesNotInDb(items map[string]filedb.Uuid) error {
	err := fastReadDirDirs(m.syncDir, func(p string, isDir bool, continueDescending *bool) {
		*continueDescending = true
		relPath := strings.TrimPrefix(p, m.syncDir+"/")
		if _, ok := items[relPath]; !ok {
			*continueDescending = false
			log.Info().Msgf("Removing local file not in database: %s", p)
			err := executer.Current.RemovePath(p)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to remove local file: %s", p)
			}
		}
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *FilenMirror) applyDiffItems(diffItems chan filedb.DiffItem, remoteDb *filedb.FileTree) {
	var wg sync.WaitGroup

	for item := range diffItems {
		var needReensure bool
		var reensurePath string
		var reensureUuid filedb.Uuid

		switch item := item.(type) {
		case filedb.DiffAdded:
			needReensure = true
			reensurePath = item.Path
			reensureUuid = item.Uuid
		case filedb.DiffRemoved:
			localPath := m.syncDir + "/" + item.Path
			log.Info().Msgf("Removing local file due to diff: %s", localPath)
			err := executer.Current.RemovePath(localPath)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to remove local file: %s", localPath)
			}
			m.osDb.Remove(item.Uuid)
		case filedb.DiffModified:
			oldLocalPath := m.syncDir + "/" + item.OldPath
			newLocalPath := m.syncDir + "/" + item.NewPath
			needReensure = true
			reensurePath = item.NewPath
			reensureUuid = item.Uuid

			if oldLocalPath != newLocalPath {
				err := executer.Current.MkdirAll(path.Dir(newLocalPath))
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to create parent directories for move: %s", newLocalPath)
					continue
				}

				log.Info().Msgf("Moving file from %s to %s", oldLocalPath, newLocalPath)
				err = executer.Current.Rename(oldLocalPath, newLocalPath)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to move file from %s to %s", oldLocalPath, newLocalPath)
					continue
				}
			}
			// Update the database entry
			if itemNode, ok := remoteDb.GetNode(item.Uuid); ok {
				m.osDb.Move(item.Uuid, itemNode.Parent, itemNode.Name)
			}
		}

		if needReensure {
			wg.Add(1)
			m.taskRunner.Schedule(TaskFunc(func() error {
				defer wg.Done()
				remoteFile, _ := remoteDb.GetNode(reensureUuid)

				localPath := m.syncDir + "/" + reensurePath

				if remoteFile.IsDir {
					return executer.Current.EnsureDir(localPath)
				}

				return executer.Current.EnsureFile(localPath, remoteFile.Modtime, remoteFile.Hash.String(), func() (io.ReadCloser, error) {
					return filenextra.CreateDownloadReader(context.Background(), m.client, reensureUuid.String())
				})
			}))
		}
	}

	wg.Wait()
}

func (m *FilenMirror) fetchRemoteDb(ctx context.Context) (*filedb.FileTree, error) {

	allFiles, allDirs, err := m.client.ListRecursive(ctx, types.DirectoryInterface(m.client.BaseFolder))
	if err != nil {
		return nil, err
	}
	remoteDb := filedb.NewFileTree()

	var dbItems []filedb.FileTreeNode
	for _, dir := range allDirs {
		parentUuid := dir.ParentUUID
		if dir.ParentUUID == m.baseDirUuid.String() {
			parentUuid = ""
		}
		dbItems = append(dbItems, filedb.FileTreeNode{
			Uuid:   filedb.UuidFromString(dir.UUID),
			IsDir:  true,
			Parent: filedb.UuidFromString(parentUuid),
			Name:   filedb.FileNameFromString(dir.Name),
		})
	}

	for _, file := range allFiles {
		parentUuid := file.ParentUUID
		if file.ParentUUID == m.baseDirUuid.String() {
			parentUuid = ""
		}

		dbItems = append(dbItems, filedb.FileTreeNode{
			Uuid:    filedb.UuidFromString(file.UUID),
			IsDir:   false,
			Parent:  filedb.UuidFromString(parentUuid),
			Modtime: file.LastModified,
			Hash:    filedb.HashFromString(file.Hash),
			Name:    filedb.FileNameFromString(file.Name),
		})
	}

	remoteDb.EnsureItems(dbItems)

	return remoteDb, nil
}

func (m *FilenMirror) Start() {
	m.taskRunner.Start(4)

	m.fullSync()
	m.filenEventListener.Start()
	go m.runFilenEventHandler()
	go m.runPeriodicFullSync()
}

func (m *FilenMirror) fullSync() {
	for {
		err := m.fullSyncOnce(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("Initial full sync failed")
		} else {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func (m *FilenMirror) runPeriodicFullSync() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		<-ticker.C
		m.fullSync()
	}
}

func (m *FilenMirror) runFilenEventHandler() {
	for {
		evt, ok := m.filenEventListener.NextEvent()
		if !ok {
			break
		}

		switch e := evt.Data.(type) {
		case *filenextra.EventSocketFileNew:
			m.ensureLocalFile(
				filedb.UuidFromString(e.UUID),
				filedb.UuidFromString(e.Parent),
				e.Meta["name"].(string),
				time.Unix(maybeString(e.Meta, "lastModified"), 0),
				"",
			)
		case *filenextra.EventSocketFileDeletedPermanent:
			m.removeLocalFile(filedb.UuidFromString(e.UUID))
		case *filenextra.EventSocketFileTrash:
			m.removeLocalFile(filedb.UuidFromString(e.UUID))
		case *filenextra.EventSocketFileRename:
			parent, ok := m.osDb.GetNode(filedb.UuidFromString(e.UUID))
			if !ok {
				log.Warn().Msgf("Failed to get parent UUID for rename of UUID: %s", e.UUID)
				continue
			}
			m.moveLocalFile(filedb.UuidFromString(e.UUID), parent.Parent, e.Meta["name"].(string))
		case *filenextra.EventSocketFileMove:
			m.moveLocalFile(filedb.UuidFromString(e.UUID), filedb.UuidFromString(e.Parent), e.Meta["name"].(string))
		case *filenextra.EventSocketFolderTrash:
			m.removeLocalFile(filedb.UuidFromString(e.UUID))
		case *filenextra.EventSocketFolderRename:
			parent, ok := m.osDb.GetNode(filedb.UuidFromString(e.UUID))
			if !ok {
				log.Warn().Msgf("Failed to get parent UUID for rename of UUID: %s", e.UUID)
				continue
			}
			m.moveLocalFile(filedb.UuidFromString(e.UUID), parent.Uuid, e.Name.Name)
		case *filenextra.EventSocketFolderMove:
			m.moveLocalFile(filedb.UuidFromString(e.UUID), filedb.UuidFromString(e.Parent), e.Name.Name)
		case *filenextra.EventSocketFolderSubCreated:
			parent := e.Parent
			if filedb.UuidFromString(e.Parent) == m.baseDirUuid {
				parent = ""
			}
			name := e.Name.Name
			parentPath, ok := m.osDb.GetPath(filedb.UuidFromString(e.Parent))
			if !ok {
				log.Warn().Msgf("Failed to get path for UUID: %s", e.UUID)
				continue
			}
			m.osDb.CreateDir(filedb.UuidFromString(e.UUID), filedb.UuidFromString(parent), name)
			localPath := m.syncDir + "/" + parentPath + "/" + name
			err := executer.Current.EnsureDir(localPath)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to create local directory: %s", localPath)
			}
		default:
			log.Info().Msgf("Unhandled event type: %s with data: %+v", evt.Name, evt.Data)
		}
	}
}

func (m *FilenMirror) removeLocalFile(uuid filedb.Uuid) {
	p, ok := m.osDb.GetPath(uuid)
	if !ok {
		log.Warn().Msgf("Failed to get path for UUID: %s", uuid)
		return
	}
	localPath := m.syncDir + "/" + p
	log.Info().Msgf("Removing local file due to permanent deletion: %s", localPath)
	err := executer.Current.RemovePath(localPath)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to remove local file: %s", localPath)
	}
	m.osDb.Remove(uuid)
}

func (m *FilenMirror) ensureLocalFile(uuid, parent filedb.Uuid, name string, modTime time.Time, hash string) {
	if parent == m.baseDirUuid {
		parent = filedb.NilUuid
	}

	parentPath, ok := m.osDb.GetPath(parent)
	if !ok {
		log.Warn().Msgf("Failed to get path for UUID: %s", uuid)
		return
	}
	localPath := m.syncDir + "/" + parentPath + "/" + name
	m.osDb.CreateFile(uuid, parent, name, modTime, hash)

	m.taskRunner.Schedule(TaskFunc(func() error {
		return executer.Current.EnsureFile(localPath, modTime, hash, func() (io.ReadCloser, error) {
			return filenextra.CreateDownloadReader(context.Background(), m.client, uuid.String())
		})
	}))
}

func (m *FilenMirror) moveLocalFile(uuid, newParent filedb.Uuid, newName string) {
	if newParent == m.baseDirUuid {
		newParent = filedb.NilUuid
	}

	oldPath, ok := m.osDb.GetPath(uuid)
	if !ok {
		log.Warn().Msgf("Failed to get old path for UUID: %s", uuid)
		return
	}
	m.osDb.Move(uuid, newParent, filedb.FileNameFromString(newName))
	newPath, ok := m.osDb.GetPath(uuid)
	if !ok {
		log.Warn().Msgf("Failed to get new path for UUID: %s", uuid)
		return
	}

	oldLocalPath := m.syncDir + "/" + oldPath
	newLocalPath := m.syncDir + "/" + newPath

	err := executer.Current.MkdirAll(path.Dir(newLocalPath))
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to create parent directories for move: %s", newLocalPath)
		return
	}

	log.Info().Msgf("Moving file from %s to %s", oldLocalPath, newLocalPath)
	err = executer.Current.Rename(oldLocalPath, newLocalPath)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to move file from %s to %s", oldLocalPath, newLocalPath)
		return
	}
}
