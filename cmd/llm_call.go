package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"text/template"

	"github.com/sashabaranov/go-openai"
	"github.com/wangii/jsonspec"
)

func llmCall[T any](sys string, tpl *template.Template, param any) (*T, error) {
	bsspec, err := jsonspec.SpecMarshal(reflect.TypeOf((*T)(nil)).Elem(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成模型OutSpec参数失败: %v", err)
	}

	spec := "```json\n" + string(bsspec) + "\n```"

	p := reflect.ValueOf(param).Elem()
	p.FieldByName("OutSpec").Set(reflect.ValueOf(spec))

	up := bytes.NewBufferString("")
	_ = tpl.Execute(up, param)

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
