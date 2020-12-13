package grpc

import "github.com/project-flogo/core/data/coerce"

type Settings struct {
	GrpcHostAddress string `md:"grpcHostAddress"`
	EnableTLS       bool   `md:"enableTLS"`
	ClientCert      string `md:"clientCert"`
}

func (s *Settings) FromMap(values map[string]interface{}) error {
	var err error

	s.GrpcHostAddress, err = coerce.ToString(values["grpcHostAddress"])
	if err != nil {
		return err
	}

	s.EnableTLS, err = coerce.ToBool(values["enableTLS"])
	if err != nil {
		return err
	}

	s.ClientCert, err = coerce.ToString(values["clientCert"])
	if err != nil {
		return err
	}

	return nil
}

func (s *Settings) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"grpcHostAddress": s.GrpcHostAddress,
		"enableTLS":       s.EnableTLS,
		"clientCert":      s.ClientCert,
	}
}

type Input struct {
	GrpcMethodParams map[string]interface{} `md:"grpcMethodParams"`
	Headers          map[string]string      `md:"headers"`
	ServiceName      string                 `md:"serviceName"`
	ProtoName        string                 `md:"protoName"`
	ProtoFile        string                 `md:"protoFile"`
	MethodName       string                 `md:"methodName"`
	Params           map[string]string      `md:"params"`
	QueryParams      map[string]string      `md:"queryParams"`
	Content          interface{}            `md:"content"`
	PathParams       map[string]string      `md:"pathParams"`
}

func (i *Input) FromMap(values map[string]interface{}) error {
	var err error

	i.ProtoName, err = coerce.ToString(values["protoName"])
	if err != nil {
		return err
	}

	i.ProtoFile, err = coerce.ToString(values["protoFile"])
	if err != nil {
		return err
	}

	i.ServiceName, err = coerce.ToString(values["serviceName"])
	if err != nil {
		return err
	}

	i.MethodName, err = coerce.ToString(values["methodName"])
	if err != nil {
		return err
	}

	return nil
}

func (i *Input) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"grpcMethodParams": i.GrpcMethodParams,
		"headers":          i.Headers,
		"serviceName":      i.ServiceName,
		"protoName":        i.ProtoName,
		"protoFile":        i.ProtoFile,
		"methodName":       i.MethodName,
		"params":           i.Params,
		"queryParams":      i.QueryParams,
		"content":          i.Content,
		"pathParams":       i.PathParams,
	}
}

// Output is the ouput from the grpc request
type Output struct {
	Body interface{} `md:"body"`
}

// FromMap converts the values from a map into the struct Output
func (o *Output) FromMap(values map[string]interface{}) error {
	o.Body = values["body"]
	return nil
}

// ToMap converts the struct Output into a map
func (o *Output) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"body": o.Body,
	}
}
