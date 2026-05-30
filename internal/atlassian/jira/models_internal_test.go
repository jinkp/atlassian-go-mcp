// models_internal_test.go tests unexported helpers in the jira package.
package jira

import (
	"testing"
)

func TestPlainTextToADF_Shape(t *testing.T) {
	adf := plainTextToADF("Fix the login page")

	// Top-level type must be "doc"
	docType, ok := adf["type"].(string)
	if !ok || docType != "doc" {
		t.Errorf("ADF top-level type: expected 'doc', got %v", adf["type"])
	}

	// Version must be 1
	version, ok := adf["version"].(int)
	if !ok || version != 1 {
		t.Errorf("ADF version: expected 1, got %v", adf["version"])
	}

	// Content must be a slice with one paragraph
	content, ok := adf["content"].([]interface{})
	if !ok || len(content) != 1 {
		t.Fatalf("ADF content: expected []interface{} with 1 item, got %T len=%d", adf["content"], len(content))
	}

	paragraph, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ADF content[0]: expected map[string]interface{}, got %T", content[0])
	}

	paraType, ok := paragraph["type"].(string)
	if !ok || paraType != "paragraph" {
		t.Errorf("ADF paragraph type: expected 'paragraph', got %v", paragraph["type"])
	}

	// Paragraph content must have the text node
	paraContent, ok := paragraph["content"].([]interface{})
	if !ok || len(paraContent) != 1 {
		t.Fatalf("ADF paragraph content: expected 1 item, got %v", paragraph["content"])
	}

	textNode, ok := paraContent[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ADF text node: expected map[string]interface{}, got %T", paraContent[0])
	}

	textType, ok := textNode["type"].(string)
	if !ok || textType != "text" {
		t.Errorf("ADF text node type: expected 'text', got %v", textNode["type"])
	}

	textVal, ok := textNode["text"].(string)
	if !ok || textVal != "Fix the login page" {
		t.Errorf("ADF text value: expected 'Fix the login page', got %v", textNode["text"])
	}
}

func TestPlainTextToADF_EmptyString(t *testing.T) {
	// Triangulate: empty string still produces valid ADF structure
	adf := plainTextToADF("")

	docType, ok := adf["type"].(string)
	if !ok || docType != "doc" {
		t.Errorf("ADF type for empty string: expected 'doc', got %v", adf["type"])
	}

	content, ok := adf["content"].([]interface{})
	if !ok || len(content) != 1 {
		t.Fatalf("ADF content for empty string: expected 1 item")
	}

	paragraph, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ADF paragraph: expected map, got %T", content[0])
	}

	paraContent, ok := paragraph["content"].([]interface{})
	if !ok || len(paraContent) != 1 {
		t.Fatalf("ADF paragraph content for empty string: expected 1 item")
	}

	textNode, ok := paraContent[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ADF text node: expected map")
	}

	textVal, ok := textNode["text"].(string)
	if !ok || textVal != "" {
		t.Errorf("ADF text for empty string: expected '', got %v", textNode["text"])
	}
}
