package main

import ("log/slog";"testing")
func TestLogLevel(t *testing.T) {
 cases:=map[string]slog.Level{"":slog.LevelInfo,"DEBUG":slog.LevelDebug,"warn":slog.LevelWarn,"error":slog.LevelError}
 for input,want:=range cases {
  got,err:=logLevel(input)
  if err!=nil || got!=want {t.Fatalf("%q: %v %v",input,got,err)}
 }
 if _,err:=logLevel("verbose");err==nil {t.Fatal("expected invalid level error")}
}
