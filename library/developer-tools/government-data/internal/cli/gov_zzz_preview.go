package cli
import("bytes";"encoding/json";"github.com/spf13/cobra")
func init() {
 registerNovelCommand(func(root *cobra.Command,f *rootFlags) {
  for _,p:=range root.Commands() {
   if p.Name()!="local-data" {continue}
   for _,c:=range p.Commands() {
    if c.Name()!="purge"||c.RunE==nil {continue}
    original:=c.RunE
    c.RunE=func(cmd *cobra.Command,args []string)error {
     if !f.dryRun {return original(cmd,args)}
     var b bytes.Buffer
     out:=cmd.OutOrStdout();cmd.SetOut(&b)
     err:=original(cmd,args);cmd.SetOut(out)
     if err!=nil {return err}
     var value map[string]any
     if json.Unmarshal(b.Bytes(),&value)==nil {value["action"]="preview local data purge";return json.NewEncoder(out).Encode(value)}
     _,err=out.Write(b.Bytes());return err
    }
   }
  }
 })
}
