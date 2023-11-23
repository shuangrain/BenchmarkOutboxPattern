package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Outbox struct {
	ID       int64  `gorm:"column:id;type:bigint(20);primaryKey;autoIncrement:true" json:"id"`
	DeviceID string `gorm:"column:device_id;type:varchar(36);not null;index:idx_device_id,priority:1" json:"device_id"`
}

type AutoIncLock struct {
	Name string `gorm:"column:name;type:varchar(36);primaryKey;" json:"name"`
}

var (
	mysql57 = "root:example@tcp(127.0.0.1:33060)/db?charset=utf8mb4&parseTime=True&loc=Local"
	mysql80 = "root:example@tcp(127.0.0.1:33061)/db?charset=utf8mb4&parseTime=True&loc=Local"
	dns     = mysql80
)

func init() {
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	err = db.AutoMigrate(&Outbox{}, &AutoIncLock{})
	if err != nil {
		panic(err)
	}
}

func truncateTables(w *gorm.DB) error {
	err := w.Raw("TRUNCATE TABLE ?", w.Model(&Outbox{}).Name()).Error
	if err != nil {
		return err
	}

	err = w.Raw("TRUNCATE TABLE ?", w.Model(&AutoIncLock{}).Name()).Error
	if err != nil {
		return err
	}

	return nil
}

func insertOutbox(w *gorm.DB) error {
	return w.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&Outbox{DeviceID: uuid.New().String()}).Error
	})
}

func insertOutboxWithOffsetLock(w *gorm.DB, name string) error {
	return w.Transaction(func(tx *gorm.DB) error {
		txErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&AutoIncLock{Name: name}).Error
		if txErr != nil {
			return txErr
		}

		return tx.Create(&Outbox{DeviceID: uuid.New().String()}).Error
	})
}

func BenchmarkInsertOutbox(b *testing.B) {
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	assert.NoError(b, err)
	defer assert.NoError(b, truncateTables(db))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, insertOutbox(db.WithContext(context.TODO())))
	}
}

func BenchmarkInsertOutboxWithParallel(b *testing.B) {
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	assert.NoError(b, err)
	defer assert.NoError(b, truncateTables(db))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			assert.NoError(b, insertOutbox(db.WithContext(context.TODO())))
		}
	})
}

func BenchmarkInsertOutboxWithOffsetLock(b *testing.B) {
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	assert.NoError(b, err)
	defer assert.NoError(b, truncateTables(db))

	name := uuid.New().String()
	err = db.Create(&AutoIncLock{Name: name}).Error
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, insertOutboxWithOffsetLock(db.WithContext(context.TODO()), name))
	}
}

func BenchmarkInsertOutboxWithOffsetLockWithParallel(b *testing.B) {
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	assert.NoError(b, err)
	defer assert.NoError(b, truncateTables(db))

	name := uuid.New().String()
	err = db.Create(&AutoIncLock{Name: name}).Error
	assert.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			assert.NoError(b, insertOutboxWithOffsetLock(db.WithContext(context.TODO()), name))
		}
	})
}
