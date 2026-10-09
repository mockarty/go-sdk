package mockarty

import (
 "os"
 "testing"
)

func TestNightSDKReadsDeletedMockLive(t *testing.T) {
 base,key,ns,id:=os.Getenv("MOCKARTY_NIGHT_BASE"),os.Getenv("MOCKARTY_NIGHT_KEY"),os.Getenv("MOCKARTY_NIGHT_NAMESPACE"),os.Getenv("MOCKARTY_NIGHT_MOCK")
 if base==""||key==""||ns==""||id=="" {t.Skip("owned native fixture required")}
 c:=NewClient(base,WithAPIKey(key),WithNamespace(ns))
 m,err:=c.Mocks().Get(t.Context(),id)
 if err!=nil{t.Fatal("SDK GET failed")}
 if m.ClosedAt<=0{t.Fatalf("SDK lost the live deletion marker: %d",m.ClosedAt)}
}
