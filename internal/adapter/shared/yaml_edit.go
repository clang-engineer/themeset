package shared

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

func decodeYAMLNode(data []byte) (*yaml.Node, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil && err != io.EOF {
		return nil, err
	}
	if document.Kind == 0 {
		document = yaml.Node{Kind: yaml.DocumentNode}
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("configuration must contain one YAML document")
		}
		return nil, err
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	if len(document.Content) == 1 && document.Content[0].Kind == yaml.ScalarNode && document.Content[0].Tag == "!!null" {
		null := document.Content[0]
		document.Content[0] = &yaml.Node{
			Kind: yaml.MappingNode, Tag: "!!map",
			HeadComment: null.HeadComment, LineComment: null.LineComment, FootComment: null.FootComment,
		}
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("configuration must be a YAML mapping")
	}
	return &document, nil
}

// EditYAML updates a YAML mapping while keeping comments and node styles on
// untouched entries. The YAML encoder may normalize indentation and spacing.
func EditYAML(path string, change func(*yaml.Node) error) error {
	data, err := Read(path, true)
	if err != nil {
		return err
	}
	document, err := decodeYAMLNode(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := change(document.Content[0]); err != nil {
		return err
	}
	data, err = yaml.Marshal(document)
	if err != nil {
		return err
	}
	return Atomic(path, data)
}

func yamlMappingValue(mapping *yaml.Node, key string) (*yaml.Node, error) {
	if mapping.Kind != yaml.MappingNode {
		return nil, errors.New("expected a YAML mapping")
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], nil
		}
	}
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return value, nil
}

func mergeYAMLNode(dst, src *yaml.Node) error {
	if dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return errors.New("YAML theme settings must be mappings")
	}
	for index := 0; index+1 < len(src.Content); index += 2 {
		key, incoming := src.Content[index], src.Content[index+1]
		var existing *yaml.Node
		for target := 0; target+1 < len(dst.Content); target += 2 {
			if dst.Content[target].Value == key.Value {
				existing = dst.Content[target+1]
				break
			}
		}
		if existing == nil {
			dst.Content = append(dst.Content, cloneYAMLNode(key), cloneYAMLNode(incoming))
			continue
		}
		if existing.Kind == yaml.MappingNode && incoming.Kind == yaml.MappingNode {
			if err := mergeYAMLNode(existing, incoming); err != nil {
				return err
			}
			continue
		}
		replacement := cloneYAMLNode(incoming)
		replacement.HeadComment = existing.HeadComment
		replacement.LineComment = existing.LineComment
		replacement.FootComment = existing.FootComment
		if replacement.Kind == existing.Kind {
			replacement.Style = existing.Style
		}
		*existing = *replacement
	}
	return nil
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		clone.Content[index] = cloneYAMLNode(child)
	}
	return &clone
}

// MergeYAMLMap merges values into a mapping without replacing existing nodes
// wholesale, retaining comments on entries that are not part of the theme.
func MergeYAMLMap(dst *yaml.Node, values map[string]any) error {
	var source yaml.Node
	if err := source.Encode(values); err != nil {
		return err
	}
	if len(source.Content) != 1 {
		return errors.New("could not encode YAML theme mapping")
	}
	return mergeYAMLNode(dst, source.Content[0])
}

func yamlPath(mapping *yaml.Node, key string) (*yaml.Node, error) {
	value, err := yamlMappingValue(mapping, key)
	if err != nil {
		return nil, err
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a YAML mapping", key)
	}
	return value, nil
}
