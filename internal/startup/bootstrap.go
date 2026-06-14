package startup

import (
	"context"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"rtagent/internal/domain/persistence"
	"rtagent/internal/infrastructure/persistence/sqlite/adapters"
	"rtagent/internal/runtime/contextengine"
	"rtagent/internal/runtime/events"
	"rtagent/internal/runtime/execution"
	"rtagent/internal/runtime/governance"
	"rtagent/internal/runtime/worldstate"
)

type RuntimeContainer struct {
	DB              *gorm.DB
	Store           persistence.Bundle
	EventBus        *events.InProcessEventBus
	LeaseManager    *governance.LocalLeaseManager
	ContextRegistry *contextengine.LocalHandleRegistry
	Materializer    *contextengine.LocalMaterializer
	Workspace       *execution.ManagedWorkspace
	WSBuilder       *worldstate.WorldStateBuilder
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
	)
	if err != nil {
		return nil, fmt.Errorf("db migration: %w", err)
	}

	storeAdapter := adapters.NewSQLiteBundle(db)
	eventBus := events.NewInProcessEventBus(1000)

	// Wire governance
	leaseMgr := governance.NewLocalLeaseManager(storeAdapter)

	// Wire context engine
	contextRegistry := contextengine.NewLocalHandleRegistry()
	materializer := contextengine.NewLocalMaterializer(contextRegistry, storeAdapter)

	// Wire workspace
	workspace := execution.NewManagedWorkspace(workDir, storeAdapter)

	wsBuilder := worldstate.NewWorldStateBuilder(storeAdapter)
	eventBus.Subscribe("*", func(ctx context.Context, ev events.Event) {
		wsBuilder.HandleEvent(ctx, ev)
	})

	container := &RuntimeContainer{
		DB:              db,
		Store:           storeAdapter,
		EventBus:        eventBus,
		LeaseManager:    leaseMgr,
		ContextRegistry: contextRegistry,
		Materializer:    materializer,
		Workspace:       workspace,
		WSBuilder:       wsBuilder,
	}
	return container, nil
}
