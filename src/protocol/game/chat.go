package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// The live client writer at BT3025.exe:0x510280 emits a message type byte,
// sender PlayerID string, a zero-valued u16, sender callsign string, a fixed
// 22-byte sender record, text string, and trailing channel byte.
const (
	chatMessageTypeOffset  = 0
	chatSenderLengthOffset = 1
	chatSenderInfoLength   = 22
	chatSenderCallSignGap  = 2
)

const (
	minChatCallSignBytes = 4
	maxChatCallSignBytes = 15
)

// ChatSenderCallSign reads the sender field from a client-shaped 0x08 PDU.
func ChatSenderCallSign(message PDU) (string, error) {
	if message.ID != ChatMessageID {
		return "", fmt.Errorf("PDU %#x is not a chat message", message.ID)
	}
	if len(message.Body) == 0 || message.Body[chatMessageTypeOffset] != 1 {
		return "", fmt.Errorf("chat body has an unsupported message type")
	}
	if len(message.Body) < chatSenderLengthOffset+4 {
		return "", fmt.Errorf("chat body is too short to contain a sender: %d bytes", len(message.Body))
	}

	nameLength := binary.BigEndian.Uint32(message.Body[chatSenderLengthOffset : chatSenderLengthOffset+4])
	if nameLength < minChatCallSignBytes || nameLength > maxChatCallSignBytes {
		return "", fmt.Errorf("chat sender callsign length %d is outside %d-%d bytes", nameLength, minChatCallSignBytes, maxChatCallSignBytes)
	}
	nameStart := chatSenderLengthOffset + 4
	nameEnd := nameStart + int(nameLength)
	if nameEnd > len(message.Body)-1 { // The channel byte is always last.
		return "", fmt.Errorf("chat sender callsign length %d exceeds the %d-byte body", nameLength, len(message.Body))
	}
	name := string(message.Body[nameStart:nameEnd])
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("chat sender callsign is not valid UTF-8")
	}
	return name, nil
}

// ChatMessageForCallSign replaces the sender field with the callsign tied to
// the connected session. All other serialized fields, including message text
// and the trailing channel byte, are preserved byte-for-byte.
func ChatMessageForCallSign(message PDU, callSign string) (PDU, error) {
	if len(callSign) < minChatCallSignBytes || len(callSign) > maxChatCallSignBytes {
		return PDU{}, fmt.Errorf("session callsign length %d is outside %d-%d bytes", len(callSign), minChatCallSignBytes, maxChatCallSignBytes)
	}
	if !utf8.ValidString(callSign) {
		return PDU{}, fmt.Errorf("session callsign is not valid UTF-8")
	}
	if _, err := ChatSenderCallSign(message); err != nil {
		return PDU{}, err
	}

	nameStart := chatSenderLengthOffset + 4
	oldLength := int(binary.BigEndian.Uint32(message.Body[chatSenderLengthOffset : chatSenderLengthOffset+4]))
	nameEnd := nameStart + oldLength
	callSignLengthOffset := nameEnd + chatSenderCallSignGap
	callSignLength, err := chatStringLength(message.Body, callSignLengthOffset)
	if err != nil {
		return PDU{}, err
	}
	callSignStart := callSignLengthOffset + 4
	callSignEnd := callSignStart + callSignLength
	if callSignEnd+chatSenderInfoLength+5 > len(message.Body) {
		return PDU{}, fmt.Errorf("chat sender record exceeds the %d-byte body", len(message.Body))
	}
	body := make([]byte, 0, len(message.Body)-oldLength-callSignLength+2*len(callSign))
	body = append(body, message.Body[:chatSenderLengthOffset]...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(callSign)))
	body = append(body, callSign...)
	body = append(body, message.Body[nameEnd:callSignLengthOffset]...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(callSign)))
	body = append(body, callSign...)
	body = append(body, message.Body[callSignEnd:]...)
	return PDU{ID: message.ID, Body: body}, nil
}

// ChatMessageText reads the length-prefixed text from a captured client-shaped
// 0x08 PDU. The final body byte is the chat channel.
func ChatMessageText(message PDU) (string, error) {
	_, textStart, textEnd, err := chatTextOffsets(message)
	if err != nil {
		return "", err
	}
	text := string(message.Body[textStart:textEnd])
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("chat text is not valid UTF-8")
	}
	return text, nil
}

// ChatMessageForDisplay canonicalizes the sender. The native client renders
// the callsign from the sender fields, so the text must remain unchanged.
func ChatMessageForDisplay(message PDU, callSign string) (PDU, error) {
	return ChatMessageForCallSign(message, callSign)
}

func chatTextOffsets(message PDU) (lengthOffset, textStart, textEnd int, err error) {
	if _, err = ChatSenderCallSign(message); err != nil {
		return 0, 0, 0, err
	}
	nameLength := int(binary.BigEndian.Uint32(message.Body[chatSenderLengthOffset : chatSenderLengthOffset+4]))
	nameEnd := chatSenderLengthOffset + 4 + nameLength
	callSignLengthOffset := nameEnd + chatSenderCallSignGap
	callSignLength, parseErr := chatStringLength(message.Body, callSignLengthOffset)
	if parseErr != nil {
		return 0, 0, 0, parseErr
	}
	callSignEnd := callSignLengthOffset + 4 + callSignLength
	lengthOffset = callSignEnd + chatSenderInfoLength
	if lengthOffset+4 > len(message.Body)-1 {
		return 0, 0, 0, fmt.Errorf("chat body is too short to contain its text length")
	}
	textStart = lengthOffset + 4
	textLength := binary.BigEndian.Uint32(message.Body[lengthOffset : lengthOffset+4])
	if uint64(textLength) != uint64(len(message.Body)-1-textStart) {
		return 0, 0, 0, fmt.Errorf("chat text length %d does not match %d trailing bytes", textLength, len(message.Body)-1-textStart)
	}
	textEnd = textStart + int(textLength)
	return lengthOffset, textStart, textEnd, nil
}

func chatStringLength(body []byte, offset int) (int, error) {
	if offset < 0 || offset+4 > len(body)-1 {
		return 0, fmt.Errorf("chat body is too short to contain a callsign string")
	}
	length := binary.BigEndian.Uint32(body[offset : offset+4])
	if length < minChatCallSignBytes || length > maxChatCallSignBytes || uint64(offset+4)+uint64(length) > uint64(len(body)-1) {
		return 0, fmt.Errorf("chat callsign length %d is invalid for a %d-byte body", length, len(body))
	}
	return int(length), nil
}

// ChatMessageFor constructs the live client wire shape for protocol and relay
// tests. Production chat packets are relayed using their native sender record.
func ChatMessageFor(callSign, text string, channel byte) PDU {
	body := []byte{1}
	body = binary.BigEndian.AppendUint32(body, uint32(len(callSign)))
	body = append(body, callSign...)
	body = append(body, 0, 0)
	body = binary.BigEndian.AppendUint32(body, uint32(len(callSign)))
	body = append(body, callSign...)
	body = append(body, make([]byte, chatSenderInfoLength)...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(text)))
	body = append(body, text...)
	body = append(body, channel)
	return PDU{ID: ChatMessageID, Body: body}
}
