package startup

import (
	"context"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
	"github.com/None9527/RTAgent-SDK/internal/infrastructure/persistence/sqlite/adapters"
	"github.com/None9527/RTAgent-SDK/internal/runtime/contextengine"
	"github.com/None9527/RTAgent-SDK/internal/runtime/events"
	"github.com/None9527/RTAgent-SDK/internal/runtime/execution"
	"github.com/None9527/RTAgent-SDK/internal/runtime/governance"
)

type RuntimeContainer struct {
	db              *gorm.DB
	Store           persistence.Bundle
	EventBus        *events.InProcessEventBus
	LeaseManager    *governance.LocalLeaseManager
	ContextRegistry *contextengine.LocalHandleRegistry
	Materializer    *contextengine.LocalMaterializer
	Workspace       *execution.ManagedWorkspace
}

// Close shuts down the runtime container, closing the event bus and the
// underlying database connection. It is safe to call multiple times.
func (c *RuntimeContainer) Close() error {
	if c.EventBus != nil {
		c.EventBus.Close()
	}
	if c.db == nil {
		return nil
	}
	sqlDB, err := c.db.DB()
	if err != nil || sqlDB == nil {
		return err
	}
	return sqlDB.Close()
}

func BootstrapSystem(ctx context.Context, dsn string, workDir string) (*RuntimeContainer, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	err = db.AutoMigrate(
		&adapters.RunModel{},
		&adapters.ThreadModel{},
		&adapters.CheckpointModel{},
		&adapters.ActivityModel{},
		&adapters.EventModel{},
		&adapters.PermissionModel{},
		&adapters.GrantModel{},
		&adapters.LeaseModel{},
		&adapters.CapabilityModel{},
		&adapters.ToolSchemaSnapshotModel{},
		&adapters.MemoryModel{},
		&adapters.ArtifactModel{},
		&adapters.ContextHandleModel{},
	)
	if err != nil {
		return nil, fmt.Errorf("db migration: %w", err)
	}

	storeAdapter := adapters.NewSQLiteBundle(db)
	eventBus := events.NewInProcessEventBus(1000)

	// Wire governance
	leaseMgr := governance.NewLocalLeaseManager(storeAdapter)

	// Wire context engine
	contextRegistry := contextengine.NewLocalHandleRegistryWithStore(storeAdapter)
	materializer := contextengine.NewLocalMaterializer(contextRegistry, storeAdapter)

	// Wire workspace
	workspace := execution.NewManagedWorkspace(workDir, storeAdapter)

	container := &RuntimeContainer{
		db:              db,
		Store:           storeAdapter,
		EventBus:        eventBus,
		LeaseManager:    leaseMgr,
		ContextRegistry: contextRegistry,
		Materializer:    materializer,
		Workspace:       workspace,
	}
	return container, nil
}
