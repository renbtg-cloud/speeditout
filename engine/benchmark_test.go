package engine

import "testing"

// Go-only interpreter microbenchmarks: no HTTP and no LLM. They measure
// CPU overhead, NOT language quality, throughput under real users or network.
func BenchmarkInterpretFixed(b *testing.B) {
    m,err:=NewMemory("");if err!=nil{b.Fatal(err)};defer m.Close()
    e:=&Engine{Memory:m}
    req:=Request{Workspace:"synthetic",UserID:"bench",Text:"Fala pro chefe que volto segunda.",Context:map[string]string{"domain":"work"}}
    b.ReportAllocs();b.ResetTimer()
    for i:=0;i<b.N;i++ { _=e.Interpret(req) }
}
func BenchmarkInterpretReported(b *testing.B) {
    m,err:=NewMemory("");if err!=nil{b.Fatal(err)};defer m.Close()
    e:=&Engine{Memory:m}
    req:=Request{Workspace:"synthetic",UserID:"bench",Text:"Meu supervisor comentou: responda ao cliente na segunda.",Context:map[string]string{"domain":"work"}}
    b.ReportAllocs();b.ResetTimer()
    for i:=0;i<b.N;i++ { _=e.Interpret(req) }
}
func BenchmarkInterpretParallel(b *testing.B) {
    m,err:=NewMemory("");if err!=nil{b.Fatal(err)};defer m.Close()
    e:=&Engine{Memory:m}
    req:=Request{Workspace:"synthetic",UserID:"bench",Text:"Fala pro chefe que volto segunda.",Context:map[string]string{"domain":"work"}}
    b.ReportAllocs();b.ResetTimer()
    b.RunParallel(func(pb *testing.PB){for pb.Next(){ _=e.Interpret(req)}})
}
