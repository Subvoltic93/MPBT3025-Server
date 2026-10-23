#!/usr/bin/env python3
"""Summarize PDU IDs and key planet fields from the server's text packet trace."""

from __future__ import annotations

import argparse
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
import re
import sys


HEADER = re.compile(
    r"^(send|recv) type=(0x[0-9a-fA-F]+) tag=(0x[0-9a-fA-F]+) length=(\d+)\s*$"
)
HEX_ROW = re.compile(r"^[0-9a-fA-F]{8}\s+.*\|")
BYTE_TOKEN = re.compile(r"^[0-9a-fA-F]{2}$")
LABELS = {
    0x80: "ENTER_COMBAT",
    0x81: "COMBAT_DATA",
    0x83: "START_COMBAT",
    0xa9: "COUNTDOWN_TIMER",
    0x09: "CLIENT_ONLINE",
    0x08: "CHAT_MESSAGE",
    0x1D: "HOST_ONLINE",
    0x26: "LIST_ROOMS",
    0x27: "LIST_ROOMS_RESPONSE",
    0x2B: "OCCUPANT_ARRIVED",
    0x2C: "OCCUPANT_DEPARTED",
    0x3B: "UPDATE_OCCUPANTS",
    0x3C: "PLAYER_UPDATE",
    0x3D: "PROFILE_UPDATE",
    0x3E: "UPDATE_ROOM",
    0x47: "SPLIT_BUFFER",
    0x87: "CREATE_CHARACTER",
    0x88: "CREATE_CHARACTER_REPLY",
    0x91: "JOIN_PLANET",
    0x92: "JOIN_PLANET_FAILED",
    0x94: "OCCUPANT_UPDATE",
    0x97: "READY_ROOM_LIST",
    0x98: "READY_ROOM_UPDATE",
    0x99: "READY_ROOM_ADDED",
    0x9A: "READY_ROOM_REMOVED",
    0x8B: "COMBAT_CONFIGURATION_UPDATE",
    0x8E: "SEAT_OPTIONS_UPDATE",
    0x9E: "CURRENT_COMBAT",
    0xA3: "HEARTBEAT",
    0xA4: "STATIC_CONFIG_RESPONSE",
    0xA7: "REQUEST_PLANET_STATES",
    0xA8: "PLANET_STATE_LIST_UPDATE",
    0xAD: "REQUEST_LOBBY_ROOMS",
}
PLANET_FIELDS = {0x91, 0xA7, 0xAD}
CHAT_CHANNELS = {0x00: "All", 0x01: "Team", 0x02: "House"}
PLAYER_RECORD_PDUS = {0x2B, 0x2C, 0x3C, 0x94}


@dataclass
class Frame:
    direction: str
    transport_type: int
    tag: int
    declared_length: int
    payload: bytes


@dataclass
class PDU:
    direction: str
    packet_id: int
    body: bytes
    frame_length: int


def parse_frames(text: str) -> list[Frame]:
    frames: list[Frame] = []
    current: tuple[str, int, int, int] | None = None
    payload = bytearray()

    def finish() -> None:
        if current is not None:
            frames.append(Frame(*current, bytes(payload)))

    for line in text.splitlines():
        header = HEADER.match(line)
        if header:
            finish()
            current = (
                header.group(1),
                int(header.group(2), 16),
                int(header.group(3), 16),
                int(header.group(4)),
            )
            payload = bytearray()
            continue
        if current is None or not HEX_ROW.match(line):
            continue
        hex_part = line.split("|", 1)[0].split()
        for token in hex_part[1:]:
            if BYTE_TOKEN.fullmatch(token):
                payload.append(int(token, 16))
    finish()
    return frames


def decode_pdus(frames: list[Frame]) -> tuple[list[PDU], int]:
    pdus: list[PDU] = []
    undecodable = 0
    for frame in frames:
        if frame.transport_type != 0:
            continue
        offset = 0
        while offset < len(frame.payload):
            remaining = len(frame.payload) - offset
            if remaining < 6:
                undecodable += 1
                break
            size = int.from_bytes(frame.payload[offset + 2 : offset + 6], "big")
            if size < 6 or size > remaining:
                undecodable += 1
                break
            packet_id = int.from_bytes(frame.payload[offset : offset + 2], "big")
            body = frame.payload[offset + 6 : offset + size]
            pdus.append(PDU(frame.direction, packet_id, body, size))
            offset += size
    return pdus, undecodable


def describe(pdu: PDU) -> str:
    label = LABELS.get(pdu.packet_id, "unknown")
    detail = f"body={len(pdu.body)}B"
    if pdu.packet_id in PLAYER_RECORD_PDUS:
        # PlayerID is a length-prefixed Aries identity string, followed by
        # badge/alertable bytes and the length-prefixed callsign.
        if len(pdu.body) >= 4:
            id_length = int.from_bytes(pdu.body[:4], "big")
            name_offset = 4 + id_length + 2
            if name_offset + 4 <= len(pdu.body):
                identity = pdu.body[4:4 + id_length].decode("utf-8", errors="replace")
                name_length = int.from_bytes(pdu.body[name_offset:name_offset + 4], "big")
                name_start = name_offset + 4
                name_end = name_start + name_length
                if 1 <= name_length <= 64 and name_end <= len(pdu.body):
                    call_sign = pdu.body[name_start:name_end].decode("utf-8", errors="replace")
                    detail += f", player_id={identity!r}, callsign={call_sign!r}"
                else:
                    detail += ", malformed callsign field"
            else:
                detail += ", malformed PlayerID string"
        else:
            detail += ", missing PlayerID string"
    elif pdu.packet_id == 0x3E and len(pdu.body) >= 13:
        reason = int.from_bytes(pdu.body[:4], "big")
        room_number = int.from_bytes(pdu.body[5:9], "big")
        host_length = int.from_bytes(pdu.body[9:13], "big")
        host_end = 13 + host_length
        if host_end + 4 <= len(pdu.body):
            host = pdu.body[13:host_end].decode("utf-8", errors="replace")
            name_length = int.from_bytes(pdu.body[host_end:host_end + 4], "big")
            name_start = host_end + 4
            if name_start + name_length <= len(pdu.body):
                name = pdu.body[name_start:name_start + name_length].decode("utf-8", errors="replace")
                detail += f", reason={reason}, occupants_follow={bool(pdu.body[4])}, room={host!r}:{room_number}, name={name!r}"
    elif pdu.packet_id == 0x08:
        sender_offset = 7
        if len(pdu.body) >= sender_offset + 4 + 1:
            sender_length = int.from_bytes(
                pdu.body[sender_offset : sender_offset + 4], "big"
            )
            sender_start = sender_offset + 4
            sender_end = sender_start + sender_length
            if 4 <= sender_length <= 15 and sender_end <= len(pdu.body) - 1:
                sender = pdu.body[sender_start:sender_end].decode(
                    "utf-8", errors="replace"
                )
                channel = pdu.body[-1]
                channel_name = CHAT_CHANNELS.get(channel, f"unknown 0x{channel:02x}")
                detail += f", sender={sender!r}, channel={channel_name}"
            else:
                detail += ", malformed sender field"
        else:
            detail += ", missing sender field"
    elif pdu.packet_id in PLANET_FIELDS and len(pdu.body) >= 4:
        detail += f", planet={int.from_bytes(pdu.body[:4], 'big')}"
    elif pdu.packet_id == 0xA8 and len(pdu.body) >= 4:
        detail += f", entries={int.from_bytes(pdu.body[:4], 'big')}"
    return f"{pdu.direction:4} 0x{pdu.packet_id:02x} {label} ({detail})"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("trace", type=Path, help="server session trace path")
    parser.add_argument(
        "--since-byte",
        type=int,
        default=0,
        help="summarize only bytes after this saved file offset",
    )
    parser.add_argument("--max-events", type=int, default=24)
    args = parser.parse_args()

    try:
        raw = args.trace.read_bytes()
    except OSError as exc:
        print(f"cannot read trace {args.trace}: {exc}", file=sys.stderr)
        return 2
    if args.since_byte < 0 or args.since_byte > len(raw):
        print(f"--since-byte must be between 0 and {len(raw)}", file=sys.stderr)
        return 2

    frames = parse_frames(raw[args.since_byte :].decode("ascii", errors="replace"))
    pdus, undecodable = decode_pdus(frames)
    counts = Counter((pdu.direction, pdu.packet_id) for pdu in pdus)
    print(f"Trace: {args.trace}")
    print(f"Frames: {len(frames)}; game PDUs: {len(pdus)}; undecodable frames: {undecodable}")
    if args.since_byte:
        print(f"Starting at byte {args.since_byte} of {len(raw)}")

    for direction in ("recv", "send"):
        parts = [
            f"0x{packet_id:02x} {LABELS.get(packet_id, 'unknown')} x{count}"
            for (event_direction, packet_id), count in sorted(counts.items())
            if event_direction == direction
        ]
        if parts:
            print(f"{direction}: " + "; ".join(parts))

    for packet_id in (0x91, 0xAD, 0xA7, 0xA8, 0x97):
        if not any(pdu.packet_id == packet_id for pdu in pdus):
            print(f"not observed: 0x{packet_id:02x} {LABELS[packet_id]}")

    limit = max(0, args.max_events)
    print(f"Last {min(limit, len(pdus))} PDU events:")
    for pdu in pdus[-limit:] if limit else []:
        print("  " + describe(pdu))
    return 1 if undecodable else 0


if __name__ == "__main__":
    raise SystemExit(main())
