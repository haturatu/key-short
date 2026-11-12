package window

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const bridgeInterface = "io.keyshort.WindowBridge"

type kwin struct {
	conn     *dbus.Conn
	mu       sync.Mutex
	sequence uint64
	pending  map[string]chan string
}

func newKWin() (*kwin, error) {
	c, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	var present bool
	if err = c.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.KWin").Store(&present); err != nil || !present {
		c.Close()
		return nil, fmt.Errorf("KWin の D-Bus サービスがありません")
	}
	k := &kwin{conn: c, pending: map[string]chan string{}}
	if err = c.Export(k, dbus.ObjectPath("/KeyShort"), bridgeInterface); err != nil {
		c.Close()
		return nil, err
	}
	return k, nil
}

// Reply is called only by the short-lived KWin script. Unique D-Bus names and
// request tokens route replies without changing KWin settings or installing a plugin.
func (k *kwin) Reply(sender dbus.Sender, token, result string) *dbus.Error {
	var owner string
	if err := k.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, "org.kde.KWin").Store(&owner); err != nil || owner != string(sender) {
		return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"KWin sender required"})
	}
	k.mu.Lock()
	ch := k.pending[token]
	k.mu.Unlock()
	if ch != nil {
		select {
		case ch <- result:
		default:
		}
	}
	return nil
}

func (k *kwin) request(ctx context.Context, activate string) ([]Window, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	k.mu.Lock()
	k.sequence++
	token := fmt.Sprintf("key-short-%d-%d", os.Getpid(), k.sequence)
	ch := make(chan string, 1)
	k.pending[token] = ch
	k.mu.Unlock()
	defer func() { k.mu.Lock(); delete(k.pending, token); k.mu.Unlock() }()
	quote := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	script := fmt.Sprintf(`(function(){
try {
 var target=%s, found=false, result=[];
 var wins=workspace.stackingOrder;
 for(var i=0;i<wins.length;i++) {
   var w=wins[i], id=String(w.internalId || w.windowId);
   if(target && id===target) {
     w.minimized=false;
     if(w.desktops && w.desktops.length) workspace.currentDesktop=w.desktops[0];
     if(w.activities && w.activities.length) workspace.currentActivity=w.activities[0];
     if(typeof workspace.activeWindow!=='undefined') workspace.activeWindow=w;
     else workspace.activeClient=w;
     found=true;
   }
   if(w.pid>0 && w.normalWindow!==false) result.push({id:id,pid:w.pid,class:String(w.resourceClass),app_id:String(w.desktopFileName||''),active:w.active===true});
 }
 if(target&&!found) throw new Error('window is no longer available');
 callDBus(%s,'/KeyShort',%s,'Reply',%s,JSON.stringify({windows:result}));
} catch(e) {callDBus(%s,'/KeyShort',%s,'Reply',%s,JSON.stringify({error:String(e)}));}
})();`, quote(activate), quote(k.conn.Names()[0]), quote(bridgeInterface), quote(token), quote(k.conn.Names()[0]), quote(bridgeInterface), quote(token))
	f, err := os.CreateTemp("", "key-short-kwin-*.js")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.WriteString(script); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	object := k.conn.Object("org.kde.KWin", "/Scripting")
	var scriptID int32
	if err = object.CallWithContext(ctx, "org.kde.kwin.Scripting.loadScript", 0, path, token).Store(&scriptID); err != nil {
		return nil, err
	}
	if scriptID < 0 {
		return nil, fmt.Errorf("KWin スクリプトを読み込めません")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		object.CallWithContext(cleanup, "org.kde.kwin.Scripting.unloadScript", 0, token)
	}()
	scriptObject := k.conn.Object("org.kde.KWin", dbus.ObjectPath(fmt.Sprintf("/Scripting/Script%d", scriptID)))
	if err = scriptObject.CallWithContext(ctx, "org.kde.kwin.Script.run", 0).Err; err != nil {
		return nil, err
	}
	select {
	case result := <-ch:
		var response struct {
			Windows []Window `json:"windows"`
			Error   string   `json:"error"`
		}
		if err = json.Unmarshal([]byte(result), &response); err != nil {
			return nil, err
		}
		if response.Error != "" {
			return nil, fmt.Errorf("KWin: %s", response.Error)
		}
		return response.Windows, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("KWin からの応答を待てませんでした: %w", ctx.Err())
	}
}

func (k *kwin) List(ctx context.Context) ([]Window, error) { return k.request(ctx, "") }
func (k *kwin) Activate(ctx context.Context, id string) error {
	_, err := k.request(ctx, id)
	return err
}
func (k *kwin) Close() error { return k.conn.Close() }
