package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/project-flogo/flow/definition"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	//nolint:staticcheck
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

const (
	serviceName = "\nservice "
)

//client template to create grpc service support file
var registryClientTemplate = template.Must(template.New("").Parse(`// This file registers with grpc service. This file was auto-generated by mashling at
	// {{ .Timestamp }}
	package {{.Package}}
	import (
		"context"
		{{if .UnaryMethodInfo}}
		"encoding/json"
		"github.com/codelity-co/flogo-grpc-activity/support"
		{{end}}
		"errors"
		{{if .Stream}}
		"strings"
		"io"
		{{end}}
		"log"
		{{if .ServerStreamMethodInfo}}
		"github.com/imdario/mergo"
		{{end}}
		servInfo "github.com/codelity-co/flogo-grpc-activity"
		"google.golang.org/grpc"
	)
	{{$serviceName := .RegServiceName}}
	{{$protoName := .ProtoName}}
	{{$option := .Option}}
	type clientService{{$protoName}}{{$serviceName}}{{$option}} struct {
		serviceInfo *servInfo.ServiceInfo
	}
	var serviceInfo{{$protoName}}{{$serviceName}}{{$option}} = &servInfo.ServiceInfo{
		ProtoName: "{{$protoName}}",
		ServiceName: "{{$serviceName}}",
	}
	func init() {
		servInfo.ClientServiceRegistery.RegisterClientService(&clientService{{$protoName}}{{$serviceName}}{{$option}}{serviceInfo: serviceInfo{{$protoName}}{{$serviceName}}{{$option}}})
		//client requests
		key := serviceInfo{{$protoName}}{{$serviceName}}{{$option}}.ProtoName + "-" + serviceInfo{{$protoName}}{{$serviceName}}{{$option}}.ServiceName
		{{- range .AllMethodInfo }}
			servInfo.RegisterClientRequest(key+"-{{.MethodName}}", &{{.MethodReqName}}{})
		{{- end }}	
	}
	//GetRegisteredClientService returns client implimentaion stub with grpc connection
	func (cs *clientService{{$protoName}}{{$serviceName}}{{$option}}) GetRegisteredClientService(gCC *grpc.ClientConn) interface{} {
		return New{{$serviceName}}Client(gCC)
	}
	func (cs *clientService{{$protoName}}{{$serviceName}}{{$option}}) ServiceInfo() *servInfo.ServiceInfo {
		return cs.serviceInfo
	}
	func (cs *clientService{{$protoName}}{{$serviceName}}{{$option}}) InvokeMethod(reqArr map[string]interface{}) map[string]interface{} {
		clientObject := reqArr["ClientObject"].({{$serviceName}}Client)
		methodName := reqArr["MethodName"].(string)
		switch methodName {
		{{- range .AllMethodInfo }}
		case "{{.MethodName}}":
			return {{.MethodName}}(clientObject, reqArr)
		{{- end }}
		}
		resMap := make(map[string]interface{},2)
		resMap["Response"] = []byte("null")
		resMap["Error"] = errors.New("Method not Available: " + methodName)
		return resMap
	}
	{{- range .UnaryMethodInfo }}
	func {{.MethodName}}(client {{$serviceName}}Client, values interface{}) map[string]interface{} {
		req := &{{.MethodReqName}}{}
		support.AssignStructValues(req, values)
		res, err := client.{{.MethodName}}(context.Background(), req)
		b, errMarshl := json.Marshal(res)
		if errMarshl != nil {
			log.Println("Error: ", errMarshl)
			return nil
		}
		resMap := make(map[string]interface{}, 2)
		resMap["Response"] = b
		resMap["Error"] = err
		return resMap
	}
	{{- end }}
	{{- range .ServerStreamMethodInfo }}
	func {{.MethodName}}(client {{$serviceName}}Client, reqArr map[string]interface{}) map[string]interface{} {
		resMap := make(map[string]interface{}, 1)
		if reqArr["Mode"] != nil {
			mode := reqArr["Mode"].(string)
			if strings.Compare(mode,"rest-to-grpc") == 0 {
				resMap["Error"] = errors.New("streaming operation is not allowed in rest to grpc case")
				return resMap
			}
		}
		req := &{{.MethodReqName}}{}
		reqData := reqArr["reqdata"].(*{{.MethodReqName}})
		if err := mergo.Merge(req, reqData, mergo.WithOverride); err != nil {
			resMap["Error"] = errors.New("unable to merge reqData values")
			return resMap
		}
		sReq := reqArr["strmReq"].({{$serviceName}}_{{.MethodName}}Server)
		stream, err := client.{{.MethodName}}(context.Background(), req)
		if err != nil {
			log.Println("erorr while getting stream object for {{.MethodName}}:", err)
			resMap["Error"] = err
			return resMap
		}
		for {
			obj, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println("erorr occured in {{.MethodName}} Recv():", err)
				resMap["Error"] = err
				return resMap
			}
			err = sReq.Send(obj)
			if err != nil {
				log.Println("error occured in {{.MethodName}} Send():", err)
				resMap["Error"] = err
				return resMap
			}
		}
		resMap["Error"] = nil
		return resMap
	}
	{{- end }}
	{{- range .ClientStreamMethodInfo }}
	func {{.MethodName}}(client {{$serviceName}}Client, reqArr map[string]interface{}) map[string]interface{} {
		resMap := make(map[string]interface{}, 1)
		if reqArr["Mode"] != nil {
			mode := reqArr["Mode"].(string)
			if strings.Compare(mode,"rest-to-grpc") == 0 {
				resMap["Error"] = errors.New("streaming operation is not allowed in rest to grpc case")
				return resMap
			}
		}
		stream, err := client.{{.MethodName}}(context.Background())
		if err != nil {
			log.Println("erorr while getting stream object for {{.MethodName}}:", err)
			resMap["Error"] = err
			return resMap
		}
		cReq := reqArr["strmReq"].({{$serviceName}}_{{.MethodName}}Server)
		for {
			dataObj, err := cReq.Recv()
			if err == io.EOF {
				obj, err := stream.CloseAndRecv()
				if err != nil {
					log.Println("erorr occured in {{.MethodName}} CloseAndRecv():", err)
					resMap["Error"] = err
					return resMap
				}
				resMap["Error"] = cReq.SendAndClose(obj)
				return resMap
			}
			if err != nil {
				log.Println("error occured in {{.MethodName}} Recv():", err)
				resMap["Error"] = err
				return resMap
			}
			if err := stream.Send(dataObj); err != nil {
				log.Println("error while sending dataObj with client stream:", err)
				resMap["Error"] = err
				return resMap
			}
		}
	}
	{{- end }}
	{{- range .BiDiStreamMethodInfo }}
	func {{.MethodName}}(client {{$serviceName}}Client, reqArr map[string]interface{}) map[string]interface{} {
		resMap := make(map[string]interface{}, 1)
		if reqArr["Mode"] != nil {
			mode := reqArr["Mode"].(string)
			if strings.Compare(mode,"rest-to-grpc") == 0 {
				resMap["Error"] = errors.New("streaming operation is not allowed in rest to grpc case")
				return resMap
			}
		}
		bReq := reqArr["strmReq"].({{$serviceName}}_{{.MethodName}}Server)
		stream, err := client.{{.MethodName}}(context.Background())
		if err != nil {
			log.Println("error while getting stream object for {{.MethodName}}:", err)
			resMap["Error"] = err
			return resMap
		}
		waits := make(chan struct{})
		go func() {
			for {
				obj, err := bReq.Recv()
				if err == io.EOF {
					resMap["Error"] = nil
					stream.CloseSend()
					close(waits)
					return
				}
				if err != nil {
					log.Println("error occured in {{.MethodName}} bidi Recv():", err)
					resMap["Error"] = err
					close(waits)
					return
				}
				if err := stream.Send(obj); err != nil {
					log.Println("error while sending obj with stream:", err)
					resMap["Error"] = err
					close(waits)
					return
				}
			}
		}()
		waitc := make(chan struct{})
		go func() {
			for {
				obj, err := stream.Recv()
				if err == io.EOF {
					resMap["Error"] = nil
					close(waitc)
					return
				}
				if err != nil {
					log.Println("erorr occured in {{.MethodName}} stream Recv():", err)
					resMap["Error"] = err
					close(waitc)
					return
				}
				if sdErr := bReq.Send(obj); sdErr != nil {
					log.Println("error while sending obj with bidi Send():", sdErr)
					resMap["Error"] = sdErr
					close(waitc)
					return
				}
			}
		}()
		<-waitc
		<-waits
		return resMap
	}
	{{- end }}
	`))

var importContent = `package main
import (
{{range $i, $ref := . }}	_ "{{ $ref.GetPackage }}"
{{end}}
)
`

var ImportTemplate = template.Must(template.New("").Parse(importContent))

// MethodInfoTree holds method information
type MethodInfoTree struct {
	MethodName    string
	MethodReqName string
	MethodResName string
	serviceName   string
}

// ProtoData holds proto file data
type ProtoData struct {
	Timestamp              time.Time
	Package                string
	UnaryMethodInfo        []MethodInfoTree
	ClientStreamMethodInfo []MethodInfoTree
	ServerStreamMethodInfo []MethodInfoTree
	BiDiStreamMethodInfo   []MethodInfoTree
	AllMethodInfo          []MethodInfoTree
	ProtoImpPath           string
	RegServiceName         string
	ProtoName              string
	Option                 string
	Stream                 bool
}

var GRPC_CLIENT_REF = "github.com/codelity-co/flogo-grpc-activity"

func main() {
	flag.Parse()
	fmt.Println("Running build...")

	appPath, _ := os.Getwd()
	flogoJsonPath := filepath.Join(appPath, "..", "flogo.json")
	_, fileErr := os.Stat(flogoJsonPath)
	if fileErr != nil {
		// look in parent directory
		flogoJsonPath = filepath.Join(appPath, "..", "..", "flogo.json")
		_, err := os.Stat(flogoJsonPath)
		if err != nil {
			log.Println(fmt.Errorf("Cannot find flogo.json file: %s", err.Error()))
		}
	}

	log.Printf("appPath has been set to: %s\n", appPath)
	m, err := GetAllProtoFileFromgRPCClientActivity(flogoJsonPath)
	if err != nil {
		log.Println(fmt.Errorf("Ger proto file error: %s", err.Error()))
		return
	}

	fmt.Println(fmt.Sprintf("m: %v", m))

	// Generate support files
	err = GenerateSupportFiles(appPath, m)
	if err != nil {
		panic(err)
	}

	// cleanup build.go, shim_support.go and <fileName>.proto
	// os.Remove(filepath.Join(appPath, "build.go"))
	log.Println("Completed build!")
}

type FlogoApp struct {
	Imports   []string    `json:"imports,omitempty"`
	Resources []*resource `json:"resources"`
}

type resource struct {
	Id   string
	Data *definition.DefinitionRep `json:"data"`
}

type ProtoLocat struct {
	protoName            string
	protoFileName        string
	flowName             string
	activityName         string
	protoFileContentType string
	protoContent         []byte
}

func (p *ProtoLocat) GetLocation() string {
	return filepath.Join(p.flowName, p.activityName)
}

func (p *ProtoLocat) GetPackage() string {
	return filepath.ToSlash(filepath.Join("engine", p.flowName, p.activityName))
}

func (f *FlogoApp) GetRef(refAlias string) string {
	for _, ref := range f.Imports {
		if len(ref) > 0 {
			ref = strings.TrimSpace(ref)
			var alias string
			//var version string

			if strings.Index(ref, " ") > 0 {
				alias = strings.TrimSpace(ref[:strings.Index(ref, " ")])
				ref = strings.TrimSpace(ref[strings.Index(ref, " ")+1:])
			}

			if strings.Index(ref, "@") > 0 {
				//version = ref[strings.Index(ref, "@")+1:]
				ref = ref[:strings.Index(ref, "@")]
			}

			if len(alias) <= 0 {
				alias = filepath.Base(ref)
			}

			if refAlias == alias {
				return ref
			}
		}
	}
	return refAlias
}

func GetAllProtoFileFromgRPCClientActivity(flogoJsonPath string) (map[string]*ProtoLocat, error) {
	v, err := ioutil.ReadFile(flogoJsonPath)
	if err != nil {
		return nil, err
	}

	app := &FlogoApp{}
	err = json.Unmarshal(v, app)
	if err != nil {
		return nil, err
	}

	protoMap := make(map[string]*ProtoLocat)

	fmt.Println("loop all resources")
	for _, v := range app.Resources {

		//Tasks
		var protoContent []byte

		fmt.Println("loop resource tasks")

		for _, act := range v.Data.Tasks {

			fmt.Println("Get activity")

			if strings.HasPrefix(act.ActivityCfgRep.Ref, "#") {
				if app.GetRef(act.ActivityCfgRep.Ref[1:]) == GRPC_CLIENT_REF {
					//Get protco file
					loc := &ProtoLocat{flowName: strings.ToLower(v.Data.Name), activityName: strings.ToLower(act.Name)}
					if _, exists := protoMap[act.ActivityCfgRep.Settings["protoName"].(string)]; !exists {
						loc.protoName = act.ActivityCfgRep.Settings["protoName"].(string)
						loc.protoFileName = loc.protoName + ".proto"
						if protoF, okk := act.ActivityCfgRep.Settings["protoFile"].(map[string]interface{}); okk {
							loc.protoFileContentType = "content"
							// decode protoFile content
							protoContentValue := protoF["content"].(string)
							index := strings.IndexAny(protoContentValue, ",")

							var protoContent []byte
							if index > -1 {
								protoContent, _ = base64.StdEncoding.DecodeString(protoContentValue[index+1:])
							} else {
								panic("Error in proto content")
							}
							loc.protoContent = protoContent
							protoMap[loc.protoName] = loc
						} else {
							loc.protoFileContentType = "file"
							protoContent, err = ioutil.ReadFile(act.ActivityCfgRep.Settings["protoFile"].(string))
							if err != nil {
								panic(err)
							}
							loc.protoContent = protoContent
							protoMap[loc.protoName] = loc
						}
					}

				}
			}
		}

		//Error Handlers
		//Tasks
		if v.Data.ErrorHandler != nil {
			fmt.Println("Found Error Handler")
			for _, act := range v.Data.ErrorHandler.Tasks {
				if strings.HasPrefix(act.ActivityCfgRep.Ref, "#") {
					if app.GetRef(act.ActivityCfgRep.Ref[1:]) == GRPC_CLIENT_REF {
						//Get protco file
						loc := &ProtoLocat{flowName: strings.ToLower(v.Data.Name), activityName: strings.ToLower(act.Name)}
						if _, exists := protoMap[act.ActivityCfgRep.Settings["protoName"].(string)]; !exists {
							loc.protoName = act.ActivityCfgRep.Settings["protoName"].(string)
							loc.protoFileName = loc.protoName + ".proto"
							if protoF, okk := act.ActivityCfgRep.Settings["protoFile"].(map[string]interface{}); okk {
								// decode protoFile content
								protoContentValue := protoF["content"].(string)
								index := strings.IndexAny(protoContentValue, ",")
								if index > -1 {
									protoContent, _ = base64.StdEncoding.DecodeString(protoContentValue[index+1:])
								} else {
									panic("Error in proto content")
								}
								loc.protoContent = protoContent
								protoMap[loc.protoName] = loc
							} else {
								protoContent, err = ioutil.ReadFile(act.ActivityCfgRep.Settings["protoFile"].(string))
								if err != nil {
									panic(err)
								}
								loc.protoContent = protoContent
								protoMap[loc.protoName] = loc
							}
						}

					}
				}
			}
		}
	}

	return protoMap, nil

}

// GenerateSupportFiles creates auto genearted code
func GenerateSupportFiles(path string, protoMap map[string]*ProtoLocat) error {

	log.Println("Generating pb files...")
	for _, v := range protoMap {

		// err := generatePbFiles(path, k, v)
		// if err != nil {
		// 	return err
		// }

		log.Println("Getting proto data...")
		pdArr, err := getProtoData(string(v.protoContent), v.protoFileName, filepath.Join(path, v.flowName, v.activityName), v.activityName)
		if err != nil {
			return err
		}

		log.Println("pdArr: %v", pdArr)

		// refactoring streaming methods and unary methods
		pdArr = arrangeProtoData(pdArr)
		fmt.Println("Creating client support files...")
		err = generateServiceImplFile(path, pdArr, "grpcclient", v)
		if err != nil {
			return err
		}

		log.Println("Support files created.")
	}

	//Create an import file to import all generated files
	// grpcImportFile, err := os.Create(filepath.Join(path, "grpcimports.go"))
	// if err != nil {
	// 	return err
	// }

	// return ImportTemplate.Execute(grpcImportFile, protoMap)
	return nil
}

// Exec executes a command within the build context.
func Exec(dirToGenerate, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	if len(dirToGenerate) != 0 {
		cmd.Dir = dirToGenerate
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error executing command: %s \n %s", string(output), err.Error())
	}
	return nil
}

// // generatePbFiles generates stub file based on given proto
// func generatePbFiles(appPath, protoName string, loc *ProtoLocat) error {
// 	_, err := exec.LookPath("protoc")
// 	if err != nil {
// 		return fmt.Errorf("Protoc is not available: %s", err.Error())
// 	}

// 	dir2Generate := filepath.Join(appPath, loc.GetLocation())
// 	if _, err := os.Stat(dir2Generate); os.IsNotExist(err) {
// 		_ = os.MkdirAll(dir2Generate, 0775)
// 	}

// 	err = ioutil.WriteFile(filepath.Join(dir2Generate, loc.protoFileName), loc.protoContent, 0644)
// 	if err != nil {
// 		return err
// 	}

// 	// execute protoc command
// 	err = Exec(dir2Generate, "protoc", "-I", "$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/", "-I", dir2Generate, filepath.Join(dir2Generate, loc.protoFileName), "--go_out="+dir2Generate)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// arrangeProtoData refactors different types of methods from all method info list
func arrangeProtoData(pdArr []ProtoData) []ProtoData {

	for index, protoData := range pdArr {
		for _, mthdInfo := range protoData.AllMethodInfo {
			clientStrm := false
			servrStrm := false

			if strings.Contains(mthdInfo.MethodReqName, "stream ") {
				mthdInfo.MethodReqName = strings.Replace(mthdInfo.MethodReqName, "stream ", "", -1)
				clientStrm = true
				protoData.Stream = true
			}
			if strings.Contains(mthdInfo.MethodResName, "stream ") {
				mthdInfo.MethodResName = strings.Replace(mthdInfo.MethodResName, "stream ", "", -1)
				servrStrm = true
				protoData.Stream = true
			}
			if !clientStrm && !servrStrm {
				protoData.UnaryMethodInfo = append(protoData.UnaryMethodInfo, mthdInfo)
			} else if clientStrm && servrStrm {
				protoData.BiDiStreamMethodInfo = append(protoData.BiDiStreamMethodInfo, mthdInfo)
			} else if clientStrm {
				protoData.ClientStreamMethodInfo = append(protoData.ClientStreamMethodInfo, mthdInfo)
			} else if servrStrm {
				protoData.ServerStreamMethodInfo = append(protoData.ServerStreamMethodInfo, mthdInfo)
			}
		}
		pdArr[index] = protoData
	}

	return pdArr
}

// getProtoData reads proto and returns proto data present in proto file
func getProtoData(protoContent string, protoName string, protoPath string, activityName string) ([]ProtoData, error) {
	var regServiceName string
	var methodInfoList []MethodInfoTree
	var ProtodataArr []ProtoData

	tempString := protoContent
	for i := 0; i < strings.Count(protoContent, serviceName); i++ {

		//getting service declaration full string
		tempString = tempString[strings.Index(tempString, serviceName):]

		//getting entire service declaration
		temp := tempString[:strings.Index(tempString, "}")+1]

		regServiceName = strings.TrimSpace(temp[strings.Index(temp, serviceName)+len(serviceName) : strings.Index(temp, "{")])
		regServiceName = generator.CamelCase(regServiceName)
		temp = temp[strings.Index(temp, "rpc")+len("rpc"):]

		//entire rpc methods content
		methodArr := strings.Split(temp, "rpc")

		for _, mthd := range methodArr {
			methodInfo := MethodInfoTree{}
			mthdDtls := strings.Split(mthd, "(")
			methodInfo.MethodName = generator.CamelCase(strings.TrimSpace(mthdDtls[0]))
			methodInfo.MethodReqName = generator.CamelCase(strings.TrimSpace(strings.Split(mthdDtls[1], ")")[0]))
			methodInfo.MethodResName = generator.CamelCase(strings.TrimSpace(strings.Split(mthdDtls[2], ")")[0]))
			methodInfo.serviceName = regServiceName
			methodInfoList = append(methodInfoList, methodInfo)
		}
		protodata := ProtoData{
			Package:        protoName,
			AllMethodInfo:  methodInfoList,
			Timestamp:      time.Now(),
			ProtoImpPath:   protoPath,
			RegServiceName: regServiceName,
			ProtoName:      protoName,
		}

		ProtodataArr = append(ProtodataArr, protodata)
		methodInfoList = nil

		//getting next service content
		tempString = tempString[strings.Index(tempString, serviceName)+len(serviceName):]
	}

	return ProtodataArr, nil
}

// generateServiceImplFile creates implementation files supported for grpc trigger and grpc service
func generateServiceImplFile(path string, pdArr []ProtoData, option string, loc *ProtoLocat) error {
	_, fileErr := os.Stat(path)
	if fileErr != nil {
		_ = os.MkdirAll(path, os.ModePerm)
	}
	for _, pd := range pdArr {
		connectorFile := filepath.Join(path, loc.protoName+"."+pd.RegServiceName+"."+option+".grpcservice.go")
		f, err := os.Create(connectorFile)
		if err != nil {
			log.Fatal("Error: ", err)
			return err
		}
		defer f.Close()
		pd.Option = option
		err = registryClientTemplate.Execute(f, pd)
		if err != nil {
			return err
		}
	}
	return nil
}
