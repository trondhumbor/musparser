package musparser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

type musHeader struct {
	Sig               [4]byte
	LenSong           uint16
	OffSong           uint16
	PrimaryChannels   uint16
	SecondaryChannels uint16
	NumInstruments    uint16
	Reserved          uint16
}

func MusToMidi(inPath, outPath string) {
	file, err := os.Open(inPath)

	if err != nil {
		panic(err)
	}

	defer file.Close()

	const midiPercussionChannel = 9
	const musPercussionChannel = 15

	header := musHeader{}
	binary.Read(file, binary.LittleEndian, &header)

	instrumentPatches := make([]uint16, header.NumInstruments)
	binary.Read(file, binary.LittleEndian, instrumentPatches)

	fmt.Println(header)
	fmt.Println(instrumentPatches)

	read := 0
	hitEnd := false
	var delayBeforeEvent uint32 = 0
	channelVelocity := make(map[byte]byte)
	channelMap := make(map[byte]byte)
	outputBuffer := new(bytes.Buffer)

	readByte := func() byte {
		var b byte
		binary.Read(file, binary.LittleEndian, &b)
		read++
		return b
	}

	writeDeltaTime := func() {
		timeDelayVarLen := varLen(delayBeforeEvent)
		binary.Write(outputBuffer, binary.BigEndian, &timeDelayVarLen)
		delayBeforeEvent = 0
	}

	getMidiChannel := func(musChannel byte) byte {
		if musChannel == musPercussionChannel {
			return midiPercussionChannel
		}

		if c, present := channelMap[musChannel]; present {
			return c
		}
		nextChannel := len(channelMap)
		if nextChannel >= midiPercussionChannel {
			nextChannel++
		}
		channelMap[musChannel] = byte(nextChannel)

		return channelMap[musChannel]
	}

	for !hitEnd {
		b := readByte()

		action := (b >> 4) & 0b0000_0111
		channel := getMidiChannel(b & 0b0000_1111)

		switch action {
		case 0: // release note
			noteNumber := readByte() & 0b0111_1111

			writeDeltaTime()
			binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
				0x80 | channel,
				noteNumber,
				64,
			})

		case 1: // play note
			noteNumber := readByte()

			_, present := channelVelocity[channel]
			if !present {
				channelVelocity[channel] = 127
			}

			volPresent := (noteNumber>>7)&0b0000_0001 == 1
			if volPresent {
				volume := readByte()
				channelVelocity[channel] = volume
			}

			writeDeltaTime()
			binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
				0x90 | channel,
				noteNumber & 0b0111_1111,
				channelVelocity[channel],
			})

		case 2: // pitch bend
			bendAmount := uint16(readByte()) * 64 // scale factor

			writeDeltaTime()
			binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
				0xE0 | channel,
				byte(bendAmount & 0b0111_1111),
				byte(bendAmount >> 7 & 0b0111_1111),
			})

		case 3: // system event
			controller := readByte()

			c := map[byte]byte{
				10: 120, // all sounds off
				11: 123, // all notes off
				12: 126, // mono
				13: 127, // poly
				14: 121, // reset all controllers
			}

			writeDeltaTime()
			binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
				0xB0 | channel,
				c[controller],
				0,
			})

		case 4: // controller
			controllerNumber := readByte()
			value := readByte()

			if controllerNumber == 0 { // instrument change
				writeDeltaTime()
				binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
					0xC0 | channel,
					value,
				})
				break
			}

			c := map[byte]byte{
				1: 0,  // bank select
				2: 1,  // modulation
				3: 7,  // volume
				4: 10, // pan
				5: 11, // expression
				6: 91, // reverb depth
				7: 93, // chorus depth
				8: 64, // sustain pedal
				9: 67, // soft pedal
			}

			writeDeltaTime()
			binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
				0xB0 | channel,
				c[controllerNumber],
				value,
			})

		case 5: // end of measure
		// This event is unused, and only left in here for completeness.

		case 6: // finish
			hitEnd = true
			writeDeltaTime()
			binary.Write(outputBuffer, binary.LittleEndian, &[]byte{
				0xFF,
				0x2F,
				0,
			})
		}

		delay := (b >> 7) & 0b0000_0001
		if delay == 1 {
			for delay == 1 {
				delayByte := readByte()
				delayBeforeEvent = delayBeforeEvent*128 + uint32(delayByte&0b0111_1111)
				delay = (delayByte >> 7) & 0b0000_0001
			}
		}
	}

	outFile, err := os.Create(outPath)

	if err != nil {
		panic(err)
	}

	defer outFile.Close()

	// Write MIDI header
	binary.Write(outFile, binary.LittleEndian, &[]byte{77, 84, 104, 100})
	binary.Write(outFile, binary.BigEndian, uint32(6))
	binary.Write(outFile, binary.BigEndian, uint16(0))
	binary.Write(outFile, binary.BigEndian, uint16(1))
	binary.Write(outFile, binary.BigEndian, uint16(70)) // assumes 120 BPM

	binary.Write(outFile, binary.LittleEndian, &[]byte{77, 84, 114, 107})
	binary.Write(outFile, binary.BigEndian, uint32(outputBuffer.Len()))
	binary.Write(outFile, binary.LittleEndian, outputBuffer.Bytes())

	fmt.Println("done")
}

func varLen(value uint32) []byte {
	if value>>7 == 0 {
		return []byte{
			byte(value),
		}
	}

	if value>>14 == 0 {
		return []byte{
			byte(value>>7 | 0x80),
			byte(value & 0x7F),
		}
	}

	if value>>21 == 0 {
		return []byte{
			byte(value>>14 | 0x80),
			byte(value>>7 | 0x80),
			byte(value & 0x7F),
		}
	}

	return []byte{
		byte(value>>21 | 0x80),
		byte(value>>14 | 0x80),
		byte(value>>7 | 0x80),
		byte(value & 0x7F),
	}
}
