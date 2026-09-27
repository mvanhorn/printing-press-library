// Research-only, anonymous read-only HTTP probe. No credentials or cookie jar.
package main
import (
 "crypto/tls"
 "encoding/json"
 "flag"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "runtime"
 "strings"
 "time"
)
func main(){
 target:=flag.String("url","https://www.ikyu.com/00002889/?cid=20261018&cod=20261019&ppc=2&rc=1&top=plans","public accommodation GET")
 agent:=flag.String("ua","","optional ordinary client user-agent")
 http1:=flag.Bool("http1",false,"disable HTTP/2 for transport comparison")
 flag.Parse()
 parsed,err:=url.Parse(*target);if err!=nil||parsed.Scheme!="https"||parsed.Host!="www.ikyu.com"||strings.HasPrefix(parsed.Path,"/booking")||strings.HasPrefix(parsed.Path,"/search"){panic("only public Ikyu read-only GET paths allowed")}
 transport:=http.DefaultTransport.(*http.Transport).Clone()
 if *http1 {transport.ForceAttemptHTTP2=false;transport.TLSNextProto=make(map[string]func(string,*tls.Conn)http.RoundTripper)}
 client:=&http.Client{Timeout:25*time.Second,Transport:transport}
 req,err:=http.NewRequest(http.MethodGet,*target,nil);if err!=nil{panic(err)}
 if *agent!=""{req.Header.Set("User-Agent",*agent)}
 started:=time.Now();response,err:=client.Do(req)
 result:=map[string]any{"requested_url":*target,"started_at":started.UTC().Format(time.RFC3339Nano),"duration_s":time.Since(started).Seconds(),"go_version":runtime.Version(),"http1_requested":*http1,"explicit_user_agent":*agent!=""}
 if err!=nil {result["error"]=err.Error()}else{
  defer response.Body.Close();body,readErr:=io.ReadAll(io.LimitReader(response.Body,8<<20));result["duration_s"]=time.Since(started).Seconds();result["status"]=response.StatusCode;result["http_version"]=response.Proto;result["content_type"]=response.Header.Get("Content-Type");result["body_bytes"]=len(body);result["effective_url"]=response.Request.URL.String();result["has_nuxt_data"]=strings.Contains(string(body),"__NUXT_DATA__")
  if readErr!=nil{result["read_error"]=readErr.Error()};if response.StatusCode!=http.StatusOK{prefix:=string(body);if len(prefix)>120{prefix=prefix[:120]};result["error_body_prefix"]=prefix}
 }
 b,err:=json.Marshal(result);if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)};fmt.Println(string(b))
}
