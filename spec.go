package spec

import (
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"sort"
	"strings"
)

type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type Parameter struct {
	Parent      string `json:"parent"`
	Name        string `json:"name"`
	Location    string `json:"location"`
	DataType    string `json:"data_type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type ResourceContent struct {
	RqHeader      []*Parameter                 `json:"request_header"`
	RqPath        []*Parameter                 `json:"request_path"`
	RqQuery       []*Parameter                 `json:"request_query"`
	RqBody        map[string][]*Parameter      `json:"request_body"`
	RqBodyExample map[string]map[string]string `json:"request_body_example"`
	RsHeader      []*Parameter                 `json:"response_header"`
	RsBody        map[string][]*Parameter      `json:"response_body"`
	RsBodyExample map[string]map[string]string `json:"response_body_example"`
}

type Resource struct {
	ResourceDefinition string          `json:"resource_definition"`
	Description        string          `json:"description"`
	Endpoint           string          `json:"endpoint"`
	TransportProtocol  string          `json:"transport_protocol"`
	RequestVerb        string          `json:"request_verb"`
	ResourceContent    ResourceContent `json:"resource_content"`
}

type Design struct {
	Info      Info        `json:"info"`
	Resources []*Resource `json:"resources"`
}

func getAuthentication(swagger *openapi3.Swagger) []*Parameter {
	var parameters []*Parameter

	for _, value := range swagger.Components.SecuritySchemes {
		parameter := new(Parameter)

		parameter.Name = value.Value.Name
		parameter.Location = value.Value.In
		parameter.DataType = value.Value.Type
		parameter.Required = true
		parameter.Description = value.Value.Description

		parameters = append(parameters, parameter)
	}

	return parameters
}

func getPathParameter(swagger *openapi3.Swagger, endpoint string) []*Parameter {
	var parameters []*Parameter

	for _, value := range swagger.Paths.Find(endpoint).Parameters {
		parameter := new(Parameter)

		parameter.Name = value.Value.Name
		parameter.Location = value.Value.In
		parameter.DataType = value.Value.Schema.Value.Type
		parameter.Required = value.Value.Required
		parameter.Description = value.Value.Description

		parameters = append(parameters, parameter)
	}

	return parameters
}

func getParameter(swagger *openapi3.Swagger, endpoint string, method string, location string) []*Parameter {
	var parameters []*Parameter
	var param openapi3.Parameters
	pathItem := swagger.Paths.Find(endpoint)

	switch strings.ToLower(method) {
	case "get":
		param = pathItem.Get.Parameters
	case "post":
		param = pathItem.Post.Parameters
	case "put":
		param = pathItem.Put.Parameters
	case "patch":
		param = pathItem.Patch.Parameters
	case "delete":
		param = pathItem.Delete.Parameters
	default:
		return nil
	}

	if param == nil {
		return nil
	}

	for _, value := range param {
		if strings.Compare(strings.ToLower(value.Value.In), strings.ToLower(location)) == 0 {
			parameter := new(Parameter)

			parameter.Name = value.Value.Name
			parameter.Location = strings.ToLower(value.Value.In)
			parameter.DataType = strings.ToLower(value.Value.Schema.Value.Type)
			parameter.Required = value.Value.Required
			parameter.Description = value.Value.Description

			parameters = append(parameters, parameter)
		}
	}

	return parameters
}

func contains(array []string, key string) bool {
	for _, value := range array {
		if strings.Compare(value, key) == 0 {
			return true
		}
	}

	return false
}

func setNode(schema *openapi3.SchemaRef) *Parameter {
	parameter := new(Parameter)
	parameter.Name = schema.Value.Title
	parameter.DataType = strings.ToLower(schema.Value.Type)
	parameter.Location = "body"
	parameter.Required = !schema.Value.Nullable
	parameter.Description = schema.Value.Description

	return parameter
}

func setPlain(schema *openapi3.SchemaRef) *Parameter {
	parameter := new(Parameter)
	parameter.Name = schema.Value.Title
	parameter.DataType = strings.ToLower(schema.Value.Type)
	parameter.Location = "body"
	parameter.Required = !schema.Value.Nullable
	parameter.Description = schema.Value.Description

	return parameter
}

func traverse(schema *openapi3.SchemaRef, parent string) (parameters []*Parameter) {
	switch strings.ToLower(schema.Value.Type) {
	case "object":
		parameter := setNode(schema)
		parameter.Parent = parent
		parameters = append(parameters, parameter)
		var properties []*openapi3.SchemaRef
		for key, property := range schema.Value.Properties {
			property.Value.Title = key
			property.Value.Nullable = !contains(schema.Value.Required, key)
			properties = append(properties, property)
		}

		sort.Slice(properties, func(i, j int) bool {
			if properties[i].Value.Type == "object" || properties[i].Value.Type == "array" {
				return false
			}
			return true
		})

		for _, property := range properties {
			parameters = append(parameters, traverse(property, parameter.Name)...)
		}
	case "array":
		parameter := setNode(schema)
		parameter.Parent = parent
		parameter.DataType = fmt.Sprintf("array[%s]", schema.Value.Items.Value.Type)
		parameters = append(parameters, parameter)

		if strings.Compare(schema.Value.Items.Value.Type, "object") == 0 {
			var properties []*openapi3.SchemaRef
			for key, property := range schema.Value.Items.Value.Properties {
				property.Value.Title = key
				property.Value.Nullable = !contains(schema.Value.Items.Value.Required, key)
				properties = append(properties, property)
			}

			sort.Slice(properties, func(i, j int) bool {
				if properties[i].Value.Type == "object" || properties[i].Value.Type == "array" {
					return false
				}
				return true
			})

			for _, property := range properties {
				parameters = append(parameters, traverse(property, parameter.Name)...)
			}
		}
	case "string":
		fallthrough
	case "number":
		fallthrough
	case "integer":
		fallthrough
	case "boolean":
		parameter := setPlain(schema)
		parameter.Parent = parent
		parameters = append(parameters, parameter)
	}

	return parameters
}

func getRqBody(swagger *openapi3.Swagger, endpoint string, method string) map[string][]*Parameter {
	var body *openapi3.RequestBodyRef
	pathItem := swagger.Paths.Find(endpoint)

	switch strings.ToLower(method) {
	case "get":
		body = pathItem.Get.RequestBody
	case "post":
		body = pathItem.Post.RequestBody
	case "put":
		body = pathItem.Put.RequestBody
	case "patch":
		body = pathItem.Patch.RequestBody
	case "delete":
		body = pathItem.Delete.RequestBody
	default:
		return nil
	}

	if body == nil {
		return nil
	}

	content := make(map[string][]*Parameter)
	for key, value := range body.Value.Content {
		content[key] = traverse(value.Schema, "root")
	}

	return content
}

func getRqBodyExample(swagger *openapi3.Swagger, endpoint string, method string) map[string]map[string]string {
	var body *openapi3.RequestBodyRef
	pathItem := swagger.Paths.Find(endpoint)

	switch strings.ToLower(method) {
	case "get":
		body = pathItem.Get.RequestBody
	case "post":
		body = pathItem.Post.RequestBody
	case "put":
		body = pathItem.Put.RequestBody
	case "patch":
		body = pathItem.Patch.RequestBody
	case "delete":
		body = pathItem.Delete.RequestBody
	default:
		return nil
	}

	if body == nil {
		return nil
	}

	example := make(map[string]map[string]string)
	for contentType, value := range body.Value.Content {
		exampleItem := make(map[string]string)
		for key, value := range value.Examples {
			marshalled, _ := json.Marshal(value.Value.Value)
			exampleItem[key] = string(marshalled)
			example[contentType] = exampleItem
		}
	}

	return example
}

func getRsHeader(swagger *openapi3.Swagger, endpoint string, method string) []*Parameter {
	var parameters []*Parameter
	var headers map[string]*openapi3.HeaderRef
	pathItem := swagger.Paths.Find(endpoint)

	switch strings.ToLower(method) {
	case "get":
		headers = pathItem.Get.Responses["200"].Value.Headers
	case "post":
		headers = pathItem.Post.Responses["200"].Value.Headers
	case "put":
		headers = pathItem.Put.Responses["200"].Value.Headers
	case "patch":
		headers = pathItem.Patch.Responses["200"].Value.Headers
	case "delete":
		headers = pathItem.Delete.Responses["200"].Value.Headers
	default:
		return nil
	}

	if headers == nil {
		return nil
	}

	for name, header := range headers {
		parameter := new(Parameter)
		parameter.Name = name
		parameter.Location = "header"
		parameter.DataType = header.Value.Schema.Value.Type
		parameter.Required = header.Value.Required
		parameter.Description = header.Value.Description

		parameters = append(parameters, parameter)
	}

	return parameters
}

func getRsBody(swagger *openapi3.Swagger, endpoint string, method string) map[string][]*Parameter {
	var body *openapi3.ResponseRef
	pathItem := swagger.Paths.Find(endpoint)

	switch strings.ToLower(method) {
	case "get":
		body = pathItem.Get.Responses["200"]
	case "post":
		body = pathItem.Post.Responses["200"]
	case "put":
		body = pathItem.Put.Responses["200"]
	case "patch":
		body = pathItem.Patch.Responses["200"]
	case "delete":
		body = pathItem.Delete.Responses["200"]
	default:
		return nil
	}

	if body == nil {
		return nil
	}

	content := make(map[string][]*Parameter)
	for key, value := range body.Value.Content {
		content[key] = traverse(value.Schema, "root")
	}

	return content
}

func getRsBodyExample(swagger *openapi3.Swagger, endpoint string, method string) map[string]map[string]string {
	var body openapi3.Responses
	pathItem := swagger.Paths.Find(endpoint)

	switch strings.ToLower(method) {
	case "get":
		body = pathItem.Get.Responses
	case "post":
		body = pathItem.Post.Responses
	case "put":
		body = pathItem.Put.Responses
	case "patch":
		body = pathItem.Patch.Responses
	case "delete":
		body = pathItem.Delete.Responses
	default:
		return nil
	}

	if body == nil {
		return nil
	}

	example := make(map[string]map[string]string)
	for httpCode, value := range body {
		exampleItem := make(map[string]string)
		for _, value := range value.Value.Content {
			for key, value := range value.Examples {
				marshalled, _ := json.Marshal(value.Value.Value)
				exampleItem[key] = string(marshalled)
				example[httpCode] = exampleItem
			}
		}
	}

	return example
}

func getResource(swagger *openapi3.Swagger) []*Resource {
	var resources []*Resource

	for path, value := range swagger.Paths {
		for method, value := range value.Operations() {
			resource := new(Resource)

			resource.ResourceDefinition = value.Summary
			resource.Description = value.Description
			resource.Endpoint = path
			resource.TransportProtocol = "HTTPS"
			resource.RequestVerb = strings.ToLower(method)

			resources = append(resources, resource)
		}
	}

	return resources
}

func getResourceContent(swagger *openapi3.Swagger, resources []*Resource) []*Resource {
	for _, resource := range resources {
		rqAuth := getAuthentication(swagger)
		for _, parameter := range rqAuth {
			if strings.Compare(parameter.Location, "header") == 0 {
				resource.ResourceContent.RqHeader = append(resource.ResourceContent.RqHeader, parameter)
			} else if strings.Compare(parameter.Location, "query") == 0 {
				resource.ResourceContent.RqQuery = append(resource.ResourceContent.RqQuery, parameter)
			}
		}

		rqPath := getPathParameter(swagger, resource.Endpoint)
		resource.ResourceContent.RqPath = append(resource.ResourceContent.RqPath, rqPath...)

		rqHeader := getParameter(swagger, resource.Endpoint, resource.RequestVerb, "header")
		resource.ResourceContent.RqHeader = append(resource.ResourceContent.RqHeader, rqHeader...)

		rqQuery := getParameter(swagger, resource.Endpoint, resource.RequestVerb, "query")
		resource.ResourceContent.RqQuery = append(resource.ResourceContent.RqQuery, rqQuery...)

		rqBody := getRqBody(swagger, resource.Endpoint, resource.RequestVerb)
		resource.ResourceContent.RqBody = rqBody

		rqBodyExample := getRqBodyExample(swagger, resource.Endpoint, resource.RequestVerb)
		resource.ResourceContent.RqBodyExample = rqBodyExample

		rsHeader := getRsHeader(swagger, resource.Endpoint, resource.RequestVerb)
		resource.ResourceContent.RsHeader = rsHeader

		rsBody := getRsBody(swagger, resource.Endpoint, resource.RequestVerb)
		resource.ResourceContent.RsBody = rsBody

		rsBodyExample := getRsBodyExample(swagger, resource.Endpoint, resource.RequestVerb)
		resource.ResourceContent.RsBodyExample = rsBodyExample
	}

	return resources
}

func getInfo(swagger *openapi3.Swagger) Info {
	var info Info
	info.Title = swagger.Info.Title
	info.Version = swagger.Info.Version
	info.Description = swagger.Info.Description

	return info
}

func Transform(swagger *openapi3.Swagger) Design {
	var design Design
	design.Info = getInfo(swagger)
	design.Resources = append(design.Resources, getResourceContent(swagger, getResource(swagger))...)

	return design
}
