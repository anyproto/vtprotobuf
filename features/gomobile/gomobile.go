/*
 *
 * Copyright 2020 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"fmt"
	"strings"

	"github.com/planetscale/vtprotobuf/generator"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	contextPackage = protogen.GoImportPath("context")
	grpcPackage    = protogen.GoImportPath("google.golang.org/grpc")
	codesPackage   = protogen.GoImportPath("google.golang.org/grpc/codes")
	statusPackage  = protogen.GoImportPath("google.golang.org/grpc/status")
)

// generateFileContent generates the gRPC service definitions, excluding the package statement.
func generateFileContent(gen *protogen.Plugin, file *protogen.File, g *generator.GeneratedFile) {
	if len(file.Services) == 0 {
		return
	}

	g.P("// This is a compile-time assertion to ensure that this generated file")
	g.P("// is compatible with the grpc package it is being compiled against.")
	g.P("// Requires gRPC-Go v1.32.0 or later.")
	g.P("const _ = ", grpcPackage.Ident("SupportPackageIsVersion7")) // When changing, update version number above.
	g.P()

	for i, service := range file.Services {
		generateService(gen, file, service, g, i)
	}
}

func generateHandlerSignature(g *generator.GeneratedFile, method *protogen.Method) string {
	methName := method.GoName
	var reqArgs []string
	reqArgs = append(reqArgs, g.QualifiedGoIdent(contextPackage.Ident("Context")))
	ret := ""
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		ret = "*" + g.QualifiedGoIdent(method.Output.GoIdent)
	}

	if !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "*"+g.QualifiedGoIdent(method.Input.GoIdent))
	}

	if method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient() {
		methName = "// Streams not supported ### " + methName
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func generateService(
	gen *protogen.Plugin,
	file *protogen.File,
	service *protogen.Service,
	g *generator.GeneratedFile,
	index int,
) {
	//path := fmt.Sprintf("6,%d", index) // 6 means service.

	handlerName := service.GoName
	deprecated := service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated()

	// Handler interface.
	handlerType := handlerName + "Handler"
	g.P("// ", handlerType, " is the handler API for ", handlerName, " service.")
	if deprecated {
		g.P("//")
		g.P(deprecationComment)
	}

	handlerVar := strings.ToLower(handlerType[0:1]) + handlerType[1:]
	g.P("var ", handlerVar, " ", handlerType)

	g.P("type ", handlerType, " interface {")
	for _, method := range service.Methods {
		g.P(generateHandlerSignature(g, method))
	}
	g.P("}")
	g.P()

	// Handler Unimplemented struct for forward compatability.
	if deprecated {
		g.P(deprecationComment)
	}

	// Handler registration.
	if deprecated {
		g.P(deprecationComment)
	}

	g.P("func register", handlerName, "Handler(srv ", handlerType, ") {")
	g.P(handlerVar, " = srv")
	g.P("}")
	g.P()

	// Handler handler implementations.
	var handlerNames []string
	for _, method := range service.Methods {
		hname := generateHandlerMethod(g, handlerVar, method)
		handlerNames = append(handlerNames, hname)
	}

	g.P()
	g.P("var PanicHandler func(v interface{})")
	g.P()
	g.P("func CommandAsync(cmd string, data []byte, callback func(data []byte)) {")
	g.P("go func() {")
	g.P("var cd []byte")
	g.P("switch cmd {")
	for _, method := range service.Methods {
		if method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient() {
			// not supported
			continue
		}
		methName := method.GoName

		g.P("case \"", methName, "\":")
		g.P("cd = ", methName, "(data)")
	}
	g.P(`default: log.Errorf("unknown command type: %s\n", cmd)`)
	g.P("}")
	g.P("if callback != nil { callback(cd) }")
	g.P("}()")
	g.P("}")

	g.P()

	g.P("type MessageHandler interface {")
	g.P("Handle(b []byte)")
	g.P("}")
	g.P()
	g.P("func CommandMobile(cmd string, data []byte, callback MessageHandler) {")
	g.P("CommandAsync(cmd, data, callback.Handle)")
	g.P("}")
	g.P()
	g.P()

	generateWrapper(g, handlerName, service)

	g.P()
}

// generateWrapper creates the unimplemented server struct
func generateWrapper(g *generator.GeneratedFile, handlerName string, service *protogen.Service) {
	handlerType := handlerName + "Handler"
	g.P("type ", handlerType, "Proxy struct {")
	g.P("client ", handlerType)
	g.P("interceptors []func(ctx context.Context, req any, methodName string, actualCall func(ctx context.Context, req any) (any, error)) (any, error)")
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		if method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient() {
			// not supported
			continue
		}
		generateServerMethodConcrete(g, handlerName, method)
	}
	g.P()
}

// generateServerMethodConcrete returns unimplemented methods which ensure forward compatibility
func generateServerMethodConcrete(g *generator.GeneratedFile, handlerName string, method *protogen.Method) {
	header := generateServerSignatureWithParamNames(g, method)
	g.P("func (h *", handlerName, "HandlerProxy) ", header, " {")
	methodName := method.GoName
	g.P("actualCall := func(ctx context.Context, req any) (any, error) {")
	g.P("return h.client.", methodName, "(ctx, req.(*", g.QualifiedGoIdent(method.Input.GoIdent), ")), nil")
	g.P("}")
	g.P("for _, interceptor := range h.interceptors {")
	g.P("toCall := actualCall")
	g.P("currentInterceptor := interceptor")
	g.P("actualCall = func(ctx context.Context, req any) (any, error) { return currentInterceptor(ctx, req, \"", methodName, "\", toCall) }")
	g.P("}")
	g.P("call, _ := actualCall(ctx, req)")
	g.P("return call.(*", g.QualifiedGoIdent(method.Output.GoIdent), ")")
	g.P("}")
}

// generateServerSignatureWithParamNames returns the server-side signature for a method with parameter names.
func generateServerSignatureWithParamNames(g *generator.GeneratedFile, method *protogen.Method) string {
	methName := method.GoName

	var reqArgs []string
	var ret string
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "ctx context.Context")
		var errSuffix = ""
		ret = "(*" + g.QualifiedGoIdent(method.Output.GoIdent) + errSuffix + ")"
	}
	if !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "req *"+g.QualifiedGoIdent(method.Input.GoIdent))
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func generateHandlerMethod(g *generator.GeneratedFile, handlerVar string, method *protogen.Method) string {
	methName := method.GoName
	hname := fmt.Sprintf("%s", methName)
	inType := g.QualifiedGoIdent(method.Input.GoIdent)
	outType := g.QualifiedGoIdent(method.Output.GoIdent)
	errorPrefix := "Error"
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		g.P("func ", hname, "(b []byte) (resp []byte) {")
		g.P("defer func() {")
		g.P("if PanicHandler != nil {")
		g.P("if r := recover(); r != nil {")
		g.P("resp, _ = (&", outType, "{Error: &", outType, errorPrefix, "{Code: ", outType, errorPrefix, "_UNKNOWN_ERROR, Description: \"panic recovered\"}}).MarshalVT()")
		g.P("PanicHandler(r)")
		g.P("}")
		g.P("}")
		g.P("}()")
		g.P()
		g.P("in := new(", inType, ")")
		g.P("if err := in.UnmarshalVT(b); err != nil { ")
		g.P("resp, _ = (&", outType, "{Error: &", outType, errorPrefix, "{Code: ", outType, errorPrefix, "_BAD_INPUT, Description: err.Error()}}).MarshalVT()")
		g.P("return resp")
		g.P("}")
		g.P()
		g.P("resp, _ = ", handlerVar, ".", methName, "(context.Background(), in).MarshalVT()")
		g.P("return resp")

		g.P("}")
		g.P()
		return hname
	}
	return hname
}

const deprecationComment = "// Deprecated: Do not use."

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }
