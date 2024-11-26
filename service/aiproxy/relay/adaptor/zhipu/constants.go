package zhipu

import (
	"github.com/labring/sealos/service/aiproxy/model"
	"github.com/labring/sealos/service/aiproxy/relay/relaymode"
)

var ModelList = []*model.ModelConfigItem{
	{
		Model: "chatglm_turbo",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "chatglm_pro",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "chatglm_std",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "chatglm_lite",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4-plus",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4-air",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4-airx",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4-long",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4-flashx",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4-flash",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-4v",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "glm-3-turbo",
		Type:  relaymode.ChatCompletions,
	},
	{
		Model: "embedding-2",
		Type:  relaymode.Embeddings,
	},
	{
		Model: "cogview-3",
		Type:  relaymode.ChatCompletions,
	},
}
