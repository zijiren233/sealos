package model

import (
	"fmt"
	"strings"
	"time"

	json "github.com/json-iterator/go"
	"github.com/labring/sealos/service/aiproxy/common"
	"gorm.io/gorm"
)

//nolint:revive
type ModelConfigKey string

const (
	ModelConfigMaxContextTokensKey ModelConfigKey = "max_context_tokens"
	ModelConfigMaxOutputTokensKey  ModelConfigKey = "max_output_tokens"
	ModelConfigToolChoiceKey       ModelConfigKey = "tool_choice"
	ModelConfigFunctionCallingKey  ModelConfigKey = "function_calling"
)

//nolint:revive
type ModelConfig struct {
	CreatedAt time.Time              `gorm:"index;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time              `gorm:"index;autoUpdateTime" json:"updated_at"`
	Config    map[ModelConfigKey]any `gorm:"serializer:fastjson"  json:"config"`
	Model     string                 `gorm:"primaryKey"           json:"model"`
	// relaymode/define.go
	Type        int     `json:"type"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

func (c *ModelConfig) MarshalJSON() ([]byte, error) {
	type Alias ModelConfig
	return json.Marshal(&struct {
		*Alias
		CreatedAt int64 `json:"created_at,omitempty"`
		UpdatedAt int64 `json:"updated_at,omitempty"`
	}{
		Alias:     (*Alias)(c),
		CreatedAt: c.CreatedAt.UnixMilli(),
		UpdatedAt: c.UpdatedAt.UnixMilli(),
	})
}

func GetModelConfigs(startIdx int, num int, model string) (configs []*ModelConfig, total int64, err error) {
	tx := DB.Model(&ModelConfig{})
	if model != "" {
		tx = tx.Where("model = ?", model)
	}
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	if total <= 0 {
		return nil, 0, nil
	}
	err = tx.Order("created_at desc").Limit(num).Offset(startIdx).Find(&configs).Error
	return configs, total, err
}

func GetAllModelConfigs() (configs []*ModelConfig, err error) {
	tx := DB.Model(&ModelConfig{})
	err = tx.Order("created_at desc").Find(&configs).Error
	return configs, err
}

func GetModelConfigsByModels(models []string) (configs []*ModelConfig, err error) {
	tx := DB.Model(&ModelConfig{}).Where("model IN (?)", models)
	err = tx.Order("created_at desc").Find(&configs).Error
	return configs, err
}

func GetModelConfig(model string) (*ModelConfig, error) {
	config := &ModelConfig{}
	err := DB.Model(&ModelConfig{}).Where("model = ?", model).First(config).Error
	return config, HandleNotFound(err, ErrModelConfigNotFound)
}

func SearchModelConfigs(keyword string, startIdx int, num int, model string) (configs []*ModelConfig, total int64, err error) {
	tx := DB.Model(&ModelConfig{}).Where("model LIKE ?", "%"+keyword+"%")
	if model != "" {
		tx = tx.Where("model = ?", model)
	}
	if keyword != "" {
		var conditions []string
		var values []interface{}

		if model == "" {
			if common.UsingPostgreSQL {
				conditions = append(conditions, "model ILIKE ?")
			} else {
				conditions = append(conditions, "model LIKE ?")
			}
			values = append(values, "%"+keyword+"%")
		}

		if len(conditions) > 0 {
			tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
		}
	}
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	if total <= 0 {
		return nil, 0, nil
	}
	err = tx.Order("created_at desc").Limit(num).Offset(startIdx).Find(&configs).Error
	return configs, total, err
}

func SaveModelConfig(config *ModelConfig) error {
	return DB.Save(config).Error
}

func SaveModelConfigs(configs []*ModelConfig) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, config := range configs {
			if err := tx.Save(config).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

const ErrModelConfigNotFound = "model config"

func DeleteModelConfig(model string) error {
	result := DB.Where("model = ?", model).Delete(&ModelConfig{})
	return HandleUpdateResult(result, ErrModelConfigNotFound)
}

func DeleteModelConfigsByModels(models []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.
			Where("model IN (?)", models).
			Delete(&ModelConfig{}).
			Error
	})
}
