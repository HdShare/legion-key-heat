package monitor

import (
	"log"
	"sync"

	hook "github.com/robotn/gohook"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zxc7563598/key-heat/pkg/keymap"
)

type KeyEventListener struct {
	keyChan  chan<- string
	stopChan chan struct{}
	mapper   keymap.KeyMapper
	once     sync.Once
	app      *application.App
	pressed  map[keyIdentity]struct{}
}

const (
	goHookExtendedKeyPrefix uint16 = 0x0E00
	enterScanCode           uint16 = 0x001C
	keypadEnterScancode     uint16 = goHookExtendedKeyPrefix | enterScanCode
)

type keyIdentity struct {
	rawcode uint16
	keycode uint16
}

func NewKeyEventListener(keyChan chan<- string, app *application.App) *KeyEventListener {
	return &KeyEventListener{
		keyChan:  keyChan,
		stopChan: make(chan struct{}),
		mapper:   keymap.GetGlobalMapper(),
		app:      app,
		pressed:  make(map[keyIdentity]struct{}),
	}
}

func (l *KeyEventListener) normalizeKeyName(rawcode, keycode uint16) string {
	if rawcode == 0x0D && keycode == keypadEnterScancode {
		return keymap.Key_KeypadEnter
	}
	return l.mapper.Normalize(rawcode)
}

func (l *KeyEventListener) keyID(rawcode, keycode uint16) keyIdentity {
	return keyIdentity{rawcode: rawcode, keycode: keycode}
}

// 开始监听
func (l *KeyEventListener) Start() error {
	log.Println("键盘监听启动中...")
	evChan := hook.Start()
	log.Println("键盘监听已启动")

	for {
		select {
		case <-l.stopChan:
			log.Println("键盘监听收到停止信号")
			hook.End() // 主动结束 hook
			return nil
		case ev, ok := <-evChan:
			if !ok {
				log.Println("键盘事件通道关闭")
				return nil
			}
			if ev.Kind == hook.KeyDown {
				id := l.keyID(ev.Rawcode, ev.Keycode)
				if _, exists := l.pressed[id]; exists {
					continue
				}
				l.pressed[id] = struct{}{}

				keyName := l.normalizeKeyName(ev.Rawcode, ev.Keycode)
				l.app.Event.Emit("key:pressed", map[string]any{
					"key":  keyName,
					"raw":  ev.Rawcode,
					"type": "down",
				})
				select {
				case l.keyChan <- keyName:
				default:
					log.Printf("通道满，丢弃: %s", keyName)
				}
			}
			if ev.Kind == hook.KeyUp {
				delete(l.pressed, l.keyID(ev.Rawcode, ev.Keycode))

				keyName := l.normalizeKeyName(ev.Rawcode, ev.Keycode)
				l.app.Event.Emit("key:pressed", map[string]any{
					"key":  keyName,
					"raw":  ev.Rawcode,
					"type": "up",
				})
			}
		}
	}
}

// 停止监听
func (l *KeyEventListener) Stop() {
	l.once.Do(func() {
		close(l.stopChan)
	})
}
