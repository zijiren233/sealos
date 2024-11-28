package deepseek

import (
	"github.com/labring/sealos/service/aiproxy/model"
	"github.com/labring/sealos/service/aiproxy/relay/relaymode"
)

var ModelList = []*model.ModelConfigItem{
	{
		Model:       "deepseek-chat",
		Type:        relaymode.ChatCompletions,
		InputPrice:  0.001,
		OutputPrice: 0.002,
	},
	{
		Model: "deepseek-coder",
		Type:  relaymode.ChatCompletions,
	},
}
