package grpc

import (
	"github.com/project-flogo/core/activity"

	"github.com/project-flogo/core/data"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/mapper"
	"github.com/project-flogo/core/data/property"
	"github.com/project-flogo/core/data/resolve"
	"github.com/project-flogo/core/support/log"

	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/mitchellh/mapstructure"
)

var activityMd = activity.ToMetadata(&Settings{}, &Input{}, &Output{})
var resolver = resolve.NewCompositeResolver(map[string]resolve.Resolver{
	".":        &resolve.ScopeResolver{},
	"env":      &resolve.EnvResolver{},
	"property": &property.Resolver{},
	"loop":     &resolve.LoopResolver{},
})

var clientInterfaceObj interface{}

func init() {
	_ = activity.Register(&Activity{}) //activity.Register(&Activity{}, New) to create instances using factory method 'New'
}

//New optional factory method, should be used if one activity instance per configuration is desired
func New(ctx activity.InitContext) (activity.Activity, error) {

	logger := ctx.Logger()

	s := &Settings{}
	sConfig, err := resolveObject(ctx.Settings())
	if err != nil {
		return nil, err
	}

	err = s.FromMap(sConfig)
	if err != nil {
		return nil, err
	}

	logger.Debugf("Setting: %s", s)

	act := &Activity{
		activitySettings: s,
	}

	var opts []grpc.DialOption
	logger.Debug("enableTLS: ", s.EnableTLS)
	if s.EnableTLS {
		logger.Debug("ClientCert: ", s.ClientCert)
		certificates, err := act.decodeClientCertificate(logger)
		if err != nil {
			return act, err
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(certificates) {
			return act, fmt.Errorf("credentials: failed to append certificates")
		}
		opts = []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(cp, ""))}

	} else {

		opts = []grpc.DialOption{grpc.WithInsecure()}

	}

	conn, err := getConnection(fmt.Sprintf("%v:%v", s.GrpcHost, s.GrpcPort), logger, opts)
	if err != nil {
		return act, err
	}

	act.connection = conn

	return act, nil
}

// Activity is an sample Activity that can be used as a base to create a custom activity
type Activity struct {
	activitySettings *Settings
	connection       *grpc.ClientConn
}

// Metadata returns the activity's metadata
func (a *Activity) Metadata() *activity.Metadata {
	return activityMd
}

// Eval implements api.Activity.Eval - Logs the Message
func (a *Activity) Eval(ctx activity.Context) (done bool, err error) {

	logger := ctx.Logger()

	input := &Input{}
	input.GrpcMethodParams = make(map[string]interface{})

	err = ctx.GetInputObject(input)
	if err != nil {
		logger.Error(err)
		return true, err
	}

	output := Output{}

	logger.Debugf("Input: %s", input)

	serviceName := input.ServiceName
	protoName := strings.Split(input.ProtoName, ".")[0]

	if len(serviceName) == 0 && len(protoName) == 0 {
		err = errors.New("Service name and Proto name required")
		logger.Error(err)
		return false, err
	}

	clServFlag := false

	if len(ClientServiceRegistery.ClientServices) == 0 {
		err = errors.New("gRPC Client services not registered")
		logger.Error(err)
		return false, err
	}

	for k, service := range ClientServiceRegistery.ClientServices {

		if strings.Compare(k, protoName+serviceName) == 0 {
			logger.Debugf("client service object found for proto [%v] and service [%v]", protoName, serviceName)
			clientInterfaceObj = service.GetRegisteredClientService(a.connection)
			clServFlag = true
		}

		methodName := input.MethodName

		var throwError bool

		if len(requests) > 0 {
			//For flogo case where all data comes from flogo.json
			md := metadata.New(input.Headers)
			input.GrpcMethodParams["contextdata"] = metadata.NewOutgoingContext(context.Background(), md)
			throwError = true
		}

		if input.GrpcMethodParams["contextdata"] != nil {

			inputs := make([]reflect.Value, 2)
			inputs[0] = reflect.ValueOf(input.GrpcMethodParams["contextdata"])

			if reqData, ok := input.GrpcMethodParams["reqdata"]; ok {
				inputs[1] = reflect.ValueOf(reqData)
			} else {
				inputData := make(map[string]interface{})
				for k, v := range input.GrpcMethodParams {
					if k == "serviceName" || k == "protoName" || k == "contextdata" || k == "reqdata" || k == "methodName" {
						continue
					}
					inputData[k] = v
				}
				request := GetRequest(protoName + "-" + serviceName + "-" + methodName)
				if request != nil {
					err := mapstructure.Decode(inputData, request)
					if err != nil {
						logger.Error(err)
						return true, err
					}
				}
				inputs[1] = reflect.ValueOf(request)
			}

			if len(input.Headers) > 0 {
				md := metadata.New(input.Headers)
				inputs = append(inputs, reflect.ValueOf(grpc.Header(&md)))
			}

			resultArr := reflect.ValueOf(clientInterfaceObj).MethodByName(methodName).Call(inputs)

			res := resultArr[0]
			grpcErr := resultArr[1]
			if !grpcErr.IsNil() {
				if throwError {
					err = fmt.Errorf("%v", grpcErr.Interface())
					logger.Error(err)
					return true, err
				} else {
					erroString := fmt.Sprintf("%v", grpcErr.Interface())
					logger.Error("Propagating error to calling function:", erroString)
					erroString = "{\"error\":\"true\",\"details\":{\"error\":\"" + erroString + "\"}}"
					err := json.Unmarshal([]byte(erroString), &output.Body)
					if err != nil {
						return true, err
					}
				}
			} else {
				output.Body = res.Interface()
			}

		} else {

			InvokeMethodData := make(map[string]interface{})
			InvokeMethodData["ClientObject"] = clientInterfaceObj
			InvokeMethodData["MethodName"] = input.GrpcMethodParams["methodName"]
			InvokeMethodData["reqdata"] = input.GrpcMethodParams["reqdata"]
			InvokeMethodData["strmReq"] = input.GrpcMethodParams["strmReq"]
			resMap := service.InvokeMethod(InvokeMethodData)

			if resMap["Error"] != nil {
				logger.Errorf("Error occured:%v", resMap["Error"])
				erroString := fmt.Sprintf("%v", resMap["Error"])
				erroString = "{\"error\":\"true\",\"details\":{\"error\":\"" + erroString + "\"}}"
				err := json.Unmarshal([]byte(erroString), &output.Body)
				if err != nil {
					return true, err
				}
			}

		}

	}

	if !clServFlag {
		err = fmt.Errorf("client service object not found for proto [%v] and service [%v]", protoName, serviceName)
		logger.Error(err)
		return false, err
	}

	return true, nil
}

func (a *Activity) decodeClientCertificate(logger log.Logger) ([]byte, error) {

	cert := a.activitySettings.ClientCert
	if cert == "" {
		return nil, fmt.Errorf("Certificate is Empty")
	}

	// case 1: if certificate comes from fileselctor it will be base64 encoded
	if strings.HasPrefix(cert, "{") {
		logger.Debug("Certificate received from file selector")
		certObj, err := coerce.ToObject(cert)
		if err == nil {
			certValue, ok := certObj["content"].(string)
			if !ok || certValue == "" {
				return nil, fmt.Errorf("No content found for certificate")
			}
			return base64.StdEncoding.DecodeString(strings.Split(certValue, ",")[1])
		}
		return nil, err
	}

	// case 2: if the certificate is defined as application property in the format "<encoding>,<encodedCertificateValue>"
	index := strings.IndexAny(cert, ",")
	if index > -1 {
		//some encoding is there
		logger.Debug("Certificate received from application property with encoding")
		encoding := cert[:index]
		certValue := cert[index+1:]

		if strings.EqualFold(encoding, "base64") {
			return base64.StdEncoding.DecodeString(certValue)
		}
		return nil, fmt.Errorf("Error parsing the certificate or given encoding may not be supported")
	}

	// case 3: if the certificate is defined as application property that points to a file
	if strings.HasPrefix(cert, "file://") {
		// app property pointing to a file
		logger.Debug("Certificate received from application property pointing to a file")
		fileName := cert[7:]
		return ioutil.ReadFile(fileName)
	}

	// case 4: if certificate is defined as path to a file (in oss)
	if strings.Contains(cert, "/") || strings.Contains(cert, "\\") {
		logger.Debug("Certificate received from settings as file path")
		_, err := os.Stat(cert)
		if err != nil {
			logger.Errorf("Cannot find certificate file: %s", err.Error())
		}
		return ioutil.ReadFile(cert)
	}

	logger.Debug("Certificate received from application property without encoding")
	return []byte(cert), nil
}

// getconnection returns single client connection object per hostaddress
func getConnection(hostAdds string, logger log.Logger, opts []grpc.DialOption) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(hostAdds, opts...)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return c, nil
}

func resolveObject(object map[string]interface{}) (map[string]interface{}, error) {
	var err error

	mapperFactory := mapper.NewFactory(resolver)
	valuesMapper, err := mapperFactory.NewMapper(object)
	if err != nil {
		return nil, err
	}

	objectValues, err := valuesMapper.Apply(data.NewSimpleScope(map[string]interface{}{}, nil))
	if err != nil {
		return nil, err
	}

	return objectValues, nil
}
