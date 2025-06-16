package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"text/template"

	"github.com/sashabaranov/go-openai"
	"github.com/wangii/jsonspec"
)

// Constraint that only allows struct types
// type StructType interface {
// 	struct{}
// }

func llmCall[T, P any](sys string, tpl *template.Template, param P) (*T, error) {

	p, err := jsonspec.AppendSpec[T](param)
	if err != nil {
		return nil, err
	}

	up := bytes.NewBufferString("")
	_ = tpl.Execute(up, p)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: sys,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: up.String(),
		},
	}

	log.Printf("提示词为:%v", up.String())

	// 调用模型接口
	// modelOutput, err := llm.QwenChat(messages, "qwen-max-latest")
	modelOutput, err := messages[1].Content, nil
	if err != nil {
		return nil, fmt.Errorf("模型处理失败: %v", err)
	}

	// 可选：校验返回结果是否为合法JSON
	structModelOutput, err := extractJSON(modelOutput)
	log.Printf("模型返回检验后的json输出:%v", structModelOutput)
	if err != nil {
		return nil, fmt.Errorf("模型生成json格式不符合要求: %v", err)
	}

	var result T
	err = json.Unmarshal([]byte(structModelOutput), &result)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 出错: %v", err)
	}

	return &result, nil
}
