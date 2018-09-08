package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-audio/generator/euclidean"

	"github.com/go-audio/music/theory"

	"github.com/go-audio/midi"
)

var (
	ppq          uint32 = 480
	flagFromFreq        = flag.Float64("freq", 0, "Use this frequency as the root of the generated data")
	flagFromNote        = flag.String("root", "", "Root note to use")
	flagOctave          = flag.Int("octave", 3, "Default octave to start from")
	flagIsMinor         = flag.Bool("minor", false, "should we use a minor scale")
)

func main() {
	flag.Parse()
	f, err := os.Create("output.mid")
	if err != nil {
		log.Fatal(err)
	}

	rand.Seed(int64(time.Now().Nanosecond()))

	enc := midi.NewEncoder(f, midi.SingleTrack, uint16(ppq))
	tr := enc.NewTrack()
	tr.AddAfterDelta(0, midi.CopyrightEvent("Generated by Go-Audio"))

	defer func() {
		tr.AddAfterDelta(0, midi.EndOfTrack())
		if err := enc.Write(); err != nil {
			log.Fatal(err)
		}
		f.Close()
	}()

	// generate a chord progression
	// scale
	var scale theory.ScaleName
	if *flagIsMinor {
		scale = theory.MelodicMinorScale
	} else {
		scale = theory.MajorScale
	}

	var rootInt int
	if *flagFromNote != "" {
		rootInt = midi.NotesToInt[strings.ToUpper(*flagFromNote)]
	} else if *flagFromFreq != 0 {
		rootInt = midi.FreqToNote(*flagFromFreq)
	} else {
		// random root
		rootInt = rand.Intn(12)
	}

	root := midi.Notes[rootInt]
	scaleName := fmt.Sprintf("%s %s scale", root, scale)
	fmt.Println(scaleName)
	keys, notes := theory.ScaleNotes(root, scale)
	fmt.Println("Notes in scale:", notes)
	tr.SetName(fmt.Sprintf("%s %v", scaleName, notes))
	octaveBump := 12 * (*flagOctave + 1)
	// move to the target octave
	for i := 0; i < len(keys); i++ {
		keys[i] += octaveBump
	}

	var progression []int
	if scale == theory.MajorScale {
		n := rand.Intn(len(theory.MajorProgressions))
		progression = theory.MajorProgressions[n]
	} else {
		n := rand.Intn(len(theory.MinorProgressions))
		progression = theory.MinorProgressions[n]
	}

	// playScale(tr, root, scale)
	// fmt.Println()
	playProgression(tr, root, scale, progression)
	fmt.Println()
	playScaleChords(tr, root, scale)
	euclideanImprov(tr, root, keys)
}

func euclideanImprov(tr *midi.Track, root string, keys []int) {
	var (
		isAPressed  bool
		isBPressed  bool
		wasAPressed bool
		wasBPressed bool
	)

	var key int
	key = midi.KeyInt(root, 3)
	key2 := keys[rand.Intn(len(keys))]
	// let's play 4 bars
	for i := 0; i < 4; i++ {
		beatsA := euclidean.Rhythm(rand.Intn(16), 16)
		beatsB := euclidean.Rhythm(rand.Intn(16), 16)
		var delta uint32
		for i, on := range beatsA {
			wasAPressed = isAPressed
			wasBPressed = isBPressed
			isAPressed = false
			isBPressed = false
			if on {
				isAPressed = true
				tr.AddAfterDelta(delta, midi.NoteOn(1, key, 99))
				delta = 0
			}
			if beatsB[i] {
				isBPressed = true
				tr.AddAfterDelta(delta, midi.NoteOn(1, key2, 99))
				delta = 0
			}

			if wasAPressed && !isAPressed {
				tr.AddAfterDelta(delta, midi.NoteOff(1, key))
				delta = 0
			}
			if wasBPressed && !isBPressed {
				tr.AddAfterDelta(delta, midi.NoteOff(1, key2))
				delta = 0
			}
			if isAPressed || isBPressed || wasAPressed || wasBPressed {
				delta = ppq / 4
				continue
			}

			delta += ppq / 4
		}
	}
}

func playScaleChords(tr *midi.Track, root string, scale theory.ScaleName) {
	fmt.Println("Chords in scale")
	var timeBuffer uint32
	keys, _ := theory.ScaleNotes(root, scale)
	octaveBump := 12 * 2

	start := keys[0] + octaveBump
	// move to the 3rd octave
	for i := 0; i < len(keys); i++ {
		keys[i] += octaveBump
		// keep going up
		if keys[i] < start {
			keys[i] += 12
		}
	}

	var firstChord *theory.Chord

	for chordTypeIDX := 0; chordTypeIDX < len(theory.RichScaleChords[scale][0]); chordTypeIDX++ {
		fmt.Println("Chord Type", chordTypeIDX)
		scaleChords := theory.RichScaleChords[scale]
		// play all chords in the scale
		for i, roman := range theory.RomanNumerals[scale] {
			if i > len(scaleChords) || chordTypeIDX > len(scaleChords[i]) {
				fmt.Println(roman, "not found")
				continue
			}
			chordName := fmt.Sprintf("%s%s\n",
				midi.Notes[keys[i]%12],
				scaleChords[i][chordTypeIDX])

			fmt.Printf("%s\t%s", roman, chordName)
			c := theory.NewChordFromAbbrev(chordName)
			if i == 0 {
				firstChord = c
			}
			if c == nil {
				fmt.Println("failed to find chord named", chordName)
				continue
			}

			for i, k := range c.Keys {
				tr.AddAfterDelta(timeBuffer, midi.NoteOn(1, k+octaveBump, 99))
				if i == 0 {
					timeBuffer = 0
				}
			}
			for i, k := range c.Keys {
				if i == 0 {
					timeBuffer = ppq * 2
				}
				tr.AddAfterDelta(timeBuffer, midi.NoteOff(1, k+octaveBump))
				if i == 0 {
					timeBuffer = 0
				}
			}
		}

		// back to the first chord
		for i, k := range firstChord.Keys {
			if i == 0 {
				timeBuffer = ppq * 2
			}
			tr.AddAfterDelta(0, midi.NoteOn(1, k+24, 99))
		}
		for i, k := range firstChord.Keys {
			if i == 0 {
				timeBuffer = ppq * 2
			}
			tr.AddAfterDelta(timeBuffer, midi.NoteOff(1, k+24))
			if i == 0 {
				timeBuffer = 0
			}
		}
	}
}

func playScale(tr *midi.Track, root string, scale theory.ScaleName) {
	keys, _ := theory.ScaleNotes(root, scale)
	octaveBump := 12 * 4

	start := keys[0] + octaveBump
	// move to the 3rd octave
	for i := 0; i < len(keys); i++ {
		keys[i] += octaveBump
		// keep going up
		if keys[i] < start {
			keys[i] += 12
		}
	}

	for _, k := range keys {
		tr.AddAfterDelta(0, midi.NoteOn(1, k, 99))
		tr.AddAfterDelta(ppq, midi.NoteOff(1, k))
	}

	// back to the first note
	tr.AddAfterDelta(0, midi.NoteOn(1, keys[0]+12, 99))
	tr.AddAfterDelta(ppq, midi.NoteOff(1, keys[0]+12))
}

func playProgression(tr *midi.Track, root string, scale theory.ScaleName, progression []int) {
	var timeBuffer uint32
	octaveBump := 12 * 2
	keys, _ := theory.ScaleNotes(root, scale)
	var c *theory.Chord

	// play the same chord twice
	repeatedChords := func(rate uint32) func() {
		return func() {
			repeats := int((ppq / rate) * 2)
			timeBuffer = 0
			for n := 0; n < repeats; n++ {
				// note on
				for i, k := range c.Keys {
					tr.AddAfterDelta(timeBuffer, midi.NoteOn(1, k+octaveBump, 99))
					if i == 0 {
						timeBuffer = 0
					}
				}
				// note off
				for i, k := range c.Keys {
					if i == 0 {
						timeBuffer = rate
					}
					tr.AddAfterDelta(timeBuffer, midi.NoteOff(1, k+octaveBump))
					if i == 0 {
						timeBuffer = 0
					}
				}
			}
		}
	}

	// play the same chord twice once at a lower octave then up
	downUp := func(rate uint32) func() {
		return func() {
			repeats := int((ppq / rate) * 4)
			timeBuffer = 0
			for n := 0; n < repeats; n++ {
				bump := octaveBump + ((n % 2) * 12)
				// note on
				for i, k := range c.Keys {
					if i == 0 {
						k -= 12
					}
					tr.AddAfterDelta(timeBuffer, midi.NoteOn(1, k+bump, 99))
					if i == 0 {
						timeBuffer = 0
						if n%2 == 0 {
							break
						}
					}
				}
				// note off
				for i, k := range c.Keys {
					if i == 0 {
						timeBuffer = rate
						k -= 12
					}
					tr.AddAfterDelta(timeBuffer, midi.NoteOff(1, k+bump))
					if i == 0 {
						timeBuffer = 0
						if n%2 == 0 {
							break
						}
					}
				}
			}
		}
	}

	arps := []func(){
		repeatedChords(ppq * 2),
		repeatedChords(ppq),
		downUp(ppq),
		downUp(ppq / 2)}

	fmt.Println("Chord progression:")
	for i, fn := range arps {
		for _, k := range progression {
			// testing using triad vs 7th
			var chordType int
			if k%2 == 0 {
				chordType = 1
			}
			chordName := fmt.Sprintf("%s%s\n",
				midi.Notes[keys[k]%12],
				theory.RichScaleChords[scale][k][chordType])

			if i == 0 {
				fmt.Printf("%s\t%s", theory.RomanNumerals[scale][k], chordName)
			}
			c = theory.NewChordFromAbbrev(chordName)
			if c == nil {
				fmt.Println("Couldn't find chord", chordName)
				continue
			}

			fn()
		}
	}
}
