// Command game runs the port.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	eaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/audio"
	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
	"github.com/karlbe/gnistaEngine/pkg/input"
	"github.com/karlbe/gnistaEngine/pkg/pack"
	"github.com/karlbe/gnistaEngine/pkg/render"
	"github.com/karlbe/gnistaEngine/pkg/title"
)

// Data in the code hunk: glyph tables for the clock digits, the room pictures' file names
// ($8404) and the room messages ($29FC).
const (
	clockGlyphsTop, clockGlyphsBottom = 0x7846, 0x7850
	roomPictureNames, roomPictures    = 0x842C, 11
	roomMessageTable, roomMessages    = 0xAFA2, 14
	flashPalette                      = "73D6"
)

// endingPictures are the pictures the endings show, titlePictures the ones of the title
// sequence, and titlePalettes the palette tables it shows the silhouette with ($7316...).
var (
	endingPictures = []string{"ET", "EX", "WT", "HC", "EN"}
	titlePictures  = []string{"NSILoader", "IT", "TA", "PF", "ST"}
	titlePalettes  = []string{"7316", "7356", "7396"}
)

// creditsText is where the title's scrolling credits start: lines ending in 0, and a line
// starting with 1 ends the text.
const creditsText = 0x9D82

// cString reads bytes from a up to (not including) the terminator.
func cString(p *script.Program, a int, end byte) string {
	var b []byte
	for ; a < p.Len() && p.Byte(a) != end; a++ {
		b = append(b, p.Byte(a))
	}
	return string(b)
}

type app struct {
	state      *game.State
	content    game.Content
	startX     int
	startY     int
	render     *render.Renderer
	mixer      *audio.Mixer // nil when muted
	player     *eaudio.Player
	separation float64
	src        input.Source
	maxTicks   uint64
	shot       string
	start      time.Time
	done       bool
	menuOpen   bool
	menuCursor int
	saveDir    string // <assets root>/saves; save states hold data derived from the original, so they stay with the assets
	slot       int    // the save slot F9, F10 and the menu use, 1 to numSlots
	toast      string
	toastLeft  int  // frames the toast is still shown
	saveAtEnd  int  // with -ticks: save to this slot after the last tick (0 = no)
	holdOff    bool // the title ended on a key press: fire and charge do nothing in the game until released

	music       musicPlayer         // the original's tracker or a pack's tracks; sound is on once the mixer is
	songStarted bool                // the title song has been started
	overHandled bool                // the music was stopped for the end of the game
	title       *title.Title        // the title sequence, while it is on; nil in the game
	titleOn     bool                // show the title before a game
	credits     int                 // the number of credit lines
	packSounds  map[int]audio.Entry // effects a pack replaces, by number
	titleFrames uint64              // frames the title has run, for -ticks and -shot
	frames      uint64              // frames the game has run, for -ticks and -shot (Tick stops in the ending)
}

// The stair step effects and the gain that brings them to about the level of the footsteps.
const (
	stairFirst, stairLast = 16, 18
	stairGain             = 1.0 // the original level; 4 matched the footsteps but was too loud, 2 still too loud
)

// numSlots is the number of save slots.
const numSlots = 5

// Menu entries (Esc). Not in the original.
const (
	menuResume = iota
	menuMinimap
	menuPlace
	menuImprove
	menuSlot
	menuSave
	menuLoad
	menuRestart
	menuQuit
	menuCount
)

// updateMenu handles the menu while it is open. The game stands still meanwhile.
func (a *app) updateMenu() error {
	press := inpututil.IsKeyJustPressed
	switch {
	case press(ebiten.KeyEscape):
		a.menuOpen = false
	case press(ebiten.KeyArrowUp) || press(ebiten.KeyW):
		a.menuCursor = (a.menuCursor + menuCount - 1) % menuCount
	case press(ebiten.KeyArrowDown) || press(ebiten.KeyS):
		a.menuCursor = (a.menuCursor + 1) % menuCount
	case press(ebiten.KeyEnter) || press(ebiten.KeyNumpadEnter) || press(ebiten.KeySpace) ||
		press(ebiten.KeyArrowLeft) || press(ebiten.KeyArrowRight) || press(ebiten.KeyA) || press(ebiten.KeyD):
		switch a.menuCursor {
		case menuImprove:
			a.setImprovements(!a.content.Improvements)
		case menuSlot:
			step := 1
			if press(ebiten.KeyArrowLeft) || press(ebiten.KeyA) {
				step = numSlots - 1
			}
			a.slot = (a.slot-1+step)%numSlots + 1
		case menuSave:
			a.saveSlot()
			a.menuOpen = false
		case menuLoad:
			if a.loadSlot() {
				a.menuOpen = false
			}
		case menuResume:
			a.menuOpen = false
		case menuMinimap:
			a.render.Minimap = !a.render.Minimap
		case menuPlace:
			a.render.MinimapBottom = !a.render.MinimapBottom
		case menuRestart:
			a.restart()
			a.menuOpen = false
		case menuQuit:
			return ebiten.Termination
		}
	}
	if a.mixer != nil {
		a.mixer.SetPaused(true)
	}
	return nil
}

func (a *app) menu() *render.Menu {
	if !a.menuOpen {
		return nil
	}
	onOff := map[bool]string{false: "OFF", true: "ON"}
	items := make([]string, menuCount)
	items[menuResume] = "Resume"
	items[menuMinimap] = "Minimap: " + onOff[a.render.Minimap]
	place := map[bool]string{false: "Corner", true: "Bottom"}
	items[menuPlace] = "Place: " + place[a.render.MinimapBottom]
	items[menuImprove] = "Improvements: " + onOff[a.content.Improvements]
	items[menuSlot] = fmt.Sprintf("Slot: %d", a.slot)
	items[menuSave] = "Save state"
	items[menuLoad] = "Load state"
	items[menuRestart] = "Restart"
	items[menuQuit] = "Quit"
	return &render.Menu{Title: "Menu", Items: items, Cursor: a.menuCursor}
}

// setImprovements switches the changes that are not in the original on or off, in the
// running game as well.
func (a *app) setImprovements(on bool) {
	a.content.Improvements = on
	a.state.SetImprovements(on)
	a.render.Extras = on
	if a.title != nil {
		a.title.Improve = on
	}
	if a.mixer != nil {
		a.mixer.Separation = a.stereo()
	}
}

// stereo is the spread between the channels: the Amiga's hard left and right in the
// original, softened with the improvements.
func (a *app) stereo() float64 {
	if a.content.Improvements {
		return a.separation
	}
	return 1
}

func (a *app) slotPath(n int) string {
	return filepath.Join(a.saveDir, fmt.Sprintf("slot%d.sav", n))
}

func (a *app) say(text string) {
	a.toast, a.toastLeft = text, 100
}

// saveSlot writes the game to the current slot.
func (a *app) saveSlot() {
	data, err := a.state.Save()
	if err == nil {
		if err = os.MkdirAll(a.saveDir, 0o755); err == nil {
			err = os.WriteFile(a.slotPath(a.slot), data, 0o644)
		}
	}
	if err != nil {
		log.Printf("save: %v", err)
		a.say("SAVE FAILED")
		return
	}
	a.say(fmt.Sprintf("SAVED SLOT %d", a.slot))
}

// loadSlot replaces the game with the one in the current slot, and reports success.
func (a *app) loadSlot() bool {
	data, err := os.ReadFile(a.slotPath(a.slot))
	if err != nil {
		a.say(fmt.Sprintf("SLOT %d IS EMPTY", a.slot))
		return false
	}
	s, err := game.LoadState(a.content, data)
	if err != nil {
		log.Printf("load: %v", err)
		a.say("LOAD FAILED")
		return false
	}
	a.state = s
	if a.mixer != nil {
		a.mixer.Stop()
	}
	a.startAmbient()
	a.say(fmt.Sprintf("LOADED SLOT %d", a.slot))
	return true
}

// startTitle begins the title sequence.
func (a *app) startTitle() {
	a.title, a.titleFrames, a.songStarted = title.New(a.credits, a.content.Improvements), 0, false
	a.music.SetStopped(true)
	if a.mixer != nil {
		a.mixer.Stop()
	}
}

// startAmbient starts the in-game track ($95EA), as the original does when a game begins.
func (a *app) startAmbient() {
	a.music.StartAmbient()
	a.overHandled = false
}

// tickMusic is the vertical blank interrupt's part of a game frame: the music runs unless the
// game is paused or in a room (the original switches its interrupts off there) and stops
// while an enemy is on screen ($9A88).
func (a *app) tickMusic() {
	switch {
	case a.state.Over():
		if !a.overHandled {
			a.overHandled = true
			a.music.SetStopped(true)
			if a.mixer != nil {
				a.mixer.Stop()
			}
		}
	case a.state.Paused || a.state.Room != nil:
	default:
		a.music.SetStopped(a.state.Spawner.Danger)
		a.music.Tick()
	}
}

// updateTitle runs the title for one frame. Fire (Ctrl, Enter or Space) works as in the
// original; Esc ends the title and starts the game, which is what development needs.
func (a *app) updateTitle() error {
	if a.content.Improvements && inpututil.IsKeyJustPressed(ebiten.KeyEscape) { // not in the original
		a.title.Skip()
	}
	fire := a.src.Poll().Has(input.Fire) || ebiten.IsKeyPressed(ebiten.KeyEnter) || ebiten.IsKeyPressed(ebiten.KeySpace)
	if a.title.Step >= 1 && !a.songStarted { // the song starts when the logo is done ($66FC)
		a.songStarted = true
		a.music.StartSong()
	}
	a.title.Update(fire, a.music.Finished())
	a.music.Tick()
	a.titleFrames++
	if a.title.Done() {
		a.title, a.holdOff = nil, a.content.Improvements
		if a.mixer != nil {
			a.mixer.Stop() // the original switches the audio DMA off at $6838
		}
		a.startAmbient()
		return nil
	}
	if a.maxTicks > 0 && a.titleFrames >= a.maxTicks && a.shot == "" {
		return ebiten.Termination
	}
	return nil
}

func (a *app) restart() {
	a.state = game.New(a.content, a.startX, a.startY)
	if a.mixer != nil {
		a.mixer.Stop()
	}
	a.startAmbient()
}

func (a *app) Update() error {
	if a.done {
		return ebiten.Termination
	}
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if a.title != nil {
		return a.updateTitle()
	}
	if a.menuOpen {
		return a.updateMenu()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.menuOpen, a.menuCursor = true, menuResume
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		a.render.Debug = !a.render.Debug
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		a.saveSlot()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		a.loadSlot()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		a.restart()
	}
	if a.state.Finished() { // the original starts again from the title
		a.restart()
		if a.titleOn {
			a.startTitle()
		}
	}
	in := a.src.Poll()
	if a.holdOff { // the key that ended the title (fire, or space) is still down
		if in&(input.Fire|input.Charge) == 0 {
			a.holdOff = false
		}
		in &^= input.Fire | input.Charge
	}
	a.state.Step(in)
	a.frames++
	a.tickMusic()
	a.playSounds()
	if a.mixer != nil {
		a.mixer.SetPaused(a.state.Paused)
	}
	if a.maxTicks > 0 && a.frames >= a.maxTicks && a.saveAtEnd > 0 {
		a.slot = a.saveAtEnd
		a.saveSlot()
		a.saveAtEnd = 0
	}
	if a.maxTicks > 0 && a.frames >= a.maxTicks && a.shot == "" {
		el := time.Since(a.start).Seconds()
		fmt.Printf("%d ticks in %.2f s = %.2f ticks/s\n", a.state.Tick, el, float64(a.state.Tick-1)/el)
		return ebiten.Termination
	}
	return nil
}

// startAudio reads the effect table and the NSISound bank and starts the mixer.
func (a *app) startAudio(l content.Loader) error {
	m, err := a.newMixer(l)
	if err != nil {
		return err
	}
	p, err := audio.Play(m)
	if err != nil {
		return err
	}
	a.mixer, a.player = m, p
	return nil
}

// newMixer builds the mixer without a player, and connects the music to it.
func (a *app) newMixer(l content.Loader) (*audio.Mixer, error) {
	// Without the original game there is no effect table: the pack's sounds are all there is.
	table := make([]audio.Entry, 32)
	var data []int8
	if code, err := l.Code(); err == nil {
		if table, err = audio.ParseTable(code); err != nil {
			return nil, err
		}
		bank, err := l.Sample("NSISound")
		if err != nil {
			return nil, err
		}
		data = bank.Data
	} else if len(a.packSounds) == 0 {
		return nil, err
	}
	for id, e := range a.packSounds {
		table[id] = e
	}
	m := audio.NewMixer(table, data, audio.SampleRate)
	// Not in the original: the stair steps (effects 16-18, played by op80) are about four
	// times quieter than the footsteps, which makes them practically silent.
	for id := stairFirst; id <= stairLast; id++ {
		m.SetGain(id, stairGain)
	}
	m.Separation = a.stereo()
	a.mixer = m
	if at, ok := a.music.(attacher); ok {
		at.Attach(m)
	} else if tr, ok := a.music.(*audio.Tracker); ok {
		tr.V = m
	}
	return m, nil
}

// playSounds hands the sound commands of the last tick to the mixer.
func (a *app) playSounds() {
	if a.mixer == nil {
		return
	}
	for _, c := range a.state.Sounds() {
		switch c.Op {
		case game.Load:
			a.mixer.Load(c.Channel, c.Sample)
		case game.Start:
			a.mixer.Start(c.Channel, false)
		case game.StartOnce:
			a.mixer.Start(c.Channel, true)
		}
	}
}

func (a *app) Draw(screen *ebiten.Image) {
	if a.title != nil {
		a.render.Toast, a.render.Menu = "", nil
		a.render.DrawTitle(screen, a.title)
		if a.shot != "" && a.titleFrames >= a.maxTicks && !a.done {
			a.done = true
			if err := savePNG(a.shot, a.render.Pixels()); err != nil {
				log.Print(err)
			}
		}
		return
	}
	a.render.Menu = a.menu()
	a.render.Toast = ""
	if a.toastLeft > 0 {
		a.toastLeft--
		a.render.Toast = a.toast
	}
	a.render.Draw(screen, a.state, a.content.Level)
	if a.shot != "" && a.frames >= a.maxTicks && !a.done {
		a.done = true
		if err := savePNG(a.shot, a.render.Pixels()); err != nil {
			log.Print(err)
		}
	}
}

func (a *app) Layout(int, int) (int, int) { return render.ScreenWidth, render.ScreenHeight }

func savePNG(path string, rgba []byte) error {
	img := image.NewRGBA(image.Rect(0, 0, render.ScreenWidth, render.ScreenHeight))
	copy(img.Pix, rgba)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// held is a fixed input, for runs without a keyboard.
type held input.Actions

func (h held) Poll() input.Actions { return input.Actions(h) }

var inputNames = map[string]input.Actions{
	"none": 0, "cheat": input.Cheat, "pause": input.Pause, "inv": input.ToggleInvulnerable, "speed": input.ToggleSpeed, "up": input.Up, "down": input.Down, "left": input.Left, "right": input.Right, "fire": input.Fire,
}

// sequence plays a list of inputs, each held for a number of ticks, then nothing.
type sequence struct {
	steps []input.Actions
	i     int
}

func (s *sequence) Poll() input.Actions {
	if s.i >= len(s.steps) {
		return 0
	}
	s.i++
	return s.steps[s.i-1]
}

// parseInputs reads "right:150,left+fire:10,none:50".
func parseInputs(spec string) (*sequence, error) {
	seq := &sequence{}
	for _, part := range strings.Split(spec, ",") {
		name, n, ok := strings.Cut(part, ":")
		ticks, err := strconv.Atoi(n)
		if !ok || err != nil {
			return nil, fmt.Errorf("bad input step %q", part)
		}
		var in input.Actions
		for _, k := range strings.Split(name, "+") {
			a, ok := inputNames[k]
			if !ok {
				return nil, fmt.Errorf("unknown input %q", k)
			}
			in |= a
		}
		for i := 0; i < ticks; i++ {
			seq.steps = append(seq.steps, in)
		}
	}
	return seq, nil
}

// These are set when the game is built (go build -ldflags "-X main.defaultImprovements=true
// -X main.defaultPack=packs/programmerart"). The port is a plain copy of the original unless
// asked otherwise; a build for another game can have the improvements on and a pack of its own.
var (
	defaultImprovements = "false"
	defaultPack         = ""
)

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func main() {
	root := flag.String("assets", "assets-local", "asset root directory")
	ticks := flag.Uint64("ticks", 0, "exit after this many ticks and report the measured tick rate")
	shot := flag.String("shot", "", "with -ticks: save the frame after the last tick to this PNG and exit")
	hold := flag.String("hold", "", "instead of the keyboard, hold one input: up, down, left, right or fire")
	inputs := flag.String("inputs", "", `instead of the keyboard, play inputs for a number of ticks each, e.g. "right:150,left+fire:10"`)
	sep := flag.Float64("separation", audio.DefaultSeparation, "stereo separation: 0 mono, 1 the Amiga's hard left/right")
	mute := flag.Bool("mute", false, "no sound")
	minimap := flag.Bool("minimap", true, "start with the minimap on (not in the original; also in the Esc menu)")
	bottom := flag.Bool("minimap-bottom", true, "put the minimap in the strip under the panel (false: in the corner of the play area)")
	showTitle := flag.Bool("title", true, "show the title sequence before the game (Esc skips it). Runs with -ticks, -shot, -load, -inputs or -hold start in the game unless -title is given; then -ticks and -shot count title frames")
	improve := flag.Bool("improvements", defaultImprovements == "true", "changes that are not in the original: cheats, WASD, the minimap, indicators, lift and stairs help and more (docs/improvements.md; also in the Esc menu)")
	loadN := flag.Int("load", 0, "start from this save slot (1-5), see F9/F10 and the Esc menu")
	timeLeft := flag.Int("time-left", 0, "set the countdown clock to this many seconds left (to reach the endings quickly)")
	saveN := flag.Int("save", 0, "with -ticks: save to this slot after the last tick")
	replayFile := flag.String("replay", "", "play a recorded game (the file go run ./cmd/solve writes) without a window, with -dump")
	dumpDir := flag.String("dump", "", "with -replay: save pictures of the game in this directory (keep it under assets-local: they show the original's graphics)")
	wavFile := flag.String("wav", "", "with -replay: write the sound of the game to this WAV file")
	dumpEvery := flag.Int("every", 5, "with -replay: save every n-th frame (50 frames make a second)")
	scale := flag.Int("scale", 3, "window scale")
	packDir := flag.String("pack", defaultPack, "a game pack: a directory of own assets that replace the original's (see docs/packs.md)")
	flag.Parse()
	if *packDir != "" && !filepath.IsAbs(*packDir) { // a pack next to the program is found from anywhere
		if _, err := os.Stat(*packDir); err != nil {
			if exe, err := os.Executable(); err == nil {
				if beside := filepath.Join(filepath.Dir(exe), *packDir); dirExists(beside) {
					*packDir = beside
				}
			}
		}
	}

	l := content.Loader{Root: *root}
	a, err := load(l, *packDir)
	if err != nil {
		log.Fatal(err)
	}
	a.maxTicks, a.shot, a.src = *ticks, *shot, keyboard{improve: &a.content.Improvements}
	a.setImprovements(*improve)
	if *timeLeft > 0 {
		a.state.SetTimeLeft(*timeLeft)
	}
	if *replayFile != "" { // play a recorded game without a window, saving pictures
		if *dumpDir == "" {
			log.Fatal("-replay needs -dump <directory>")
		}
		if *wavFile != "" {
			if _, err := a.newMixer(l); err != nil {
				log.Fatal(err)
			}
		}
		if err := a.dumpReplay(*replayFile, *dumpDir, *dumpEvery, *wavFile); err != nil {
			log.Fatal(err)
		}
		return
	}
	explicit := false
	flag.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "title" })
	scripted := *ticks != 0 || *shot != "" || *loadN != 0 || *inputs != "" || *hold != ""
	a.titleOn = *showTitle && (!scripted || explicit)
	if a.titleOn {
		a.startTitle()
	}
	a.saveDir, a.slot, a.saveAtEnd = filepath.Join(*root, "saves"), 1, *saveN
	if *loadN > 0 {
		if *loadN > numSlots {
			log.Fatalf("-load: slots are 1-%d", numSlots)
		}
		a.slot = *loadN
		if !a.loadSlot() {
			log.Fatalf("-load: %s", a.toast)
		}
	}
	switch {
	case *hold != "":
		in, ok := inputNames[*hold]
		if !ok {
			log.Fatalf("unknown -hold %q", *hold)
		}
		a.src = held(in)
	case *inputs != "":
		seq, err := parseInputs(*inputs)
		if err != nil {
			log.Fatal(err)
		}
		a.src = seq
	}

	a.separation = *sep
	a.render.Minimap, a.render.MinimapBottom = *minimap, *bottom
	if !*mute && *shot == "" {
		if err := a.startAudio(l); err != nil {
			log.Printf("no sound: %v", err)
		}
	}
	ebiten.SetTPS(game.TicksPerSecond)
	ebiten.SetWindowSize(render.ScreenWidth**scale, render.ScreenHeight**scale)
	ebiten.SetWindowTitle("GnistaEngine")
	if err := ebiten.RunGame(a); err != nil {
		log.Fatal(err)
	}
}

// origin is what the original game files give: the assets, the scripts, the level and the music.
// A pack that brings everything of its own runs without it.
type origin struct {
	as             render.Assets
	prog           *script.Program
	level          *world.Level
	tracker        *audio.Tracker
	credits        int
	startX, startY int
}

func loadOriginal(l content.Loader) (*origin, error) {
	o := &origin{}
	ext, err := l.Extracted()
	if err != nil {
		return nil, err
	}
	code, err := l.Code()
	if err != nil {
		return nil, err
	}
	prog, err := l.Program()
	if err != nil {
		return nil, err
	}
	level, err := l.Level()
	if err != nil {
		return nil, err
	}
	icons, err := l.Bank("NSIIcons")
	if err != nil {
		return nil, err
	}
	bobs, err := l.Bank("NSIBobs")
	if err != nil {
		return nil, err
	}
	hud, err := l.ILBM("NSIMenu")
	if err != nil {
		return nil, err
	}
	font, err := l.Font()
	if err != nil {
		return nil, err
	}
	as := &o.as
	as.Tiles, as.Bobs, as.HUD, as.Font = icons, bobs, hud, font
	for d := 0; d < 10; d++ {
		as.Clock[0][d], as.Clock[1][d] = prog.Byte(clockGlyphsTop+d), prog.Byte(clockGlyphsBottom+d)
	}
	// Room pictures are named in a table of file names; one of them (HK) is not on the disk.
	for i := 0; i < roomPictures; i++ {
		name := strings.TrimPrefix(cString(prog, prog.Long(roomPictureNames+4*i), 0), "df0:")
		img, err := l.ILBM(name)
		if err != nil {
			img = nil
		}
		as.Rooms = append(as.Rooms, img)
	}
	as.Pictures = map[string]*amiga.ILBM{}
	for _, name := range append(append([]string{}, endingPictures...), titlePictures...) {
		if as.Pictures[name], err = l.ILBM(name); err != nil {
			return nil, err
		}
	}
	as.TitlePalettes = map[string][]color.RGBA{}
	for _, key := range titlePalettes {
		for _, c := range ext.Palettes[key] {
			as.TitlePalettes[key] = append(as.TitlePalettes[key], amiga.RGBA(c))
		}
	}
	for a := creditsText; a < prog.Len() && prog.Byte(a) != 1; o.credits++ {
		line := cString(prog, a, 0)
		as.Credits = append(as.Credits, line)
		a += len(line) + 1
	}
	for _, c := range ext.Palettes[flashPalette] {
		as.Flash = append(as.Flash, amiga.RGBA(c))
	}
	for i := 0; i < roomMessages; i++ {
		as.Messages = append(as.Messages, []byte(cString(prog, prog.Long(roomMessageTable+4*i), 1)))
	}
	for _, c := range ext.Palettes[ext.Gameplay] {
		as.Palette = append(as.Palette, amiga.RGBA(c))
	}
	if o.tracker, err = loadMusic(l, code); err != nil {
		return nil, err
	}
	o.prog, o.level, o.startX, o.startY = prog, level, ext.StartX, ext.StartY
	return o, nil
}

func load(l content.Loader, packDir string) (*app, error) {
	o, err := loadOriginal(l)
	if err != nil {
		if packDir == "" {
			return nil, err
		}
		log.Printf("no original game files (%v): the pack stands alone", err)
		o = &origin{}
		o.as.Pictures = map[string]*amiga.ILBM{}
	}
	as, prog, level := o.as, o.prog, o.level
	startX, startY, credits := o.startX, o.startY, o.credits
	var music musicPlayer = nopMusic{}
	if o.tracker != nil {
		music = o.tracker
	}
	var packSounds map[int]audio.Entry
	if packDir != "" { // own assets replace the original's, piece by piece
		p, err := pack.Open(packDir)
		if err != nil {
			return nil, err
		}
		res, err := applyPack(p, &as)
		if err != nil {
			return nil, fmt.Errorf("pack %s: %w", packDir, err)
		}
		if res.Level != nil {
			level, startX, startY = res.Level, res.Level.T.StartViewX, res.Level.T.StartViewY
		}
		if res.Credits >= 0 {
			credits = res.Credits
		}
		packSounds = res.Sounds
		if res.Program != nil {
			prog = res.Program
		}
		if res.TitleTrack != nil || res.GameTrack != nil {
			music = &packMusic{orig: music, title: res.TitleTrack, game: res.GameTrack}
		}
		log.Printf("pack %q: %s", p.Manifest.Name, res.Summary)
	}
	switch {
	case prog == nil:
		return nil, fmt.Errorf("no scripts: the pack has none (scripts/*.gs and program.json) and the original game is not here")
	case level == nil:
		return nil, fmt.Errorf("no level: the pack has no level.tmj and the original game is not here")
	case len(as.Palette) == 0:
		return nil, fmt.Errorf("no palette: pack.json has none and the original game is not here")
	}
	c := game.Content{Program: prog, Level: level}
	return &app{state: game.New(c, startX, startY), content: c, startX: startX, startY: startY, render: render.New(as), credits: credits, music: music, packSounds: packSounds}, nil
}
