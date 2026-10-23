#!/usr/bin/env python3
"""Run the supplied DLL's combat serializers in an isolated i386 fixture process.

Maps PE sections at their preferred addresses; no DLL initialization, imports,
Wine clients, or network connections are used. A bounded buffer replaces the
stream's raw-write virtual method. Numeric encoding and command serialization
execute the original x86 instructions. Requires Linux i386 execution, as, ld.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[1]
CLIENT_ROOT = Path(os.environ.get("MPBT3025_CLIENT_ROOT", ROOT.parent.parent / "MPBT3025" / "client"))
DLL = CLIENT_ROOT / "MPBT3025andGILS/MPBT3025/BTCombat.dll"


def mapped_image(data):
    pe = struct.unpack_from("<I", data, 0x3C)[0]
    if data[pe:pe + 4] != b"PE\0\0":
        raise ValueError("expected PE image")
    machine, count = struct.unpack_from("<HH", data, pe + 4)
    optional_size = struct.unpack_from("<H", data, pe + 20)[0]
    optional = pe + 24
    base = struct.unpack_from("<I", data, optional + 28)[0]
    if machine != 0x14C or base != 0x69000000:
        raise ValueError("expected the supplied i386 BTCombat image")
    image = bytearray(struct.unpack_from("<I", data, optional + 56)[0])
    for index in range(count):
        section = optional + optional_size + index * 40
        virtual_size, rva, size, offset = struct.unpack_from("<IIII", data, section + 8)
        if rva + max(size, virtual_size) > len(image):
            raise ValueError("PE section exceeds image")
        image[rva:rva + size] = data[offset:offset + size]
    return image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=("title", "mech", "ready", "commando", "position", "client-position", "fire", "server-fire", "damage", "local-damage"))
    parser.add_argument("--weapon-slot", type=int, choices=range(85), help="override the synthetic fire weapon slot")
    parser.add_argument("--target-entity", type=int, choices=range(-1, 84), help="override the synthetic fire target (-1 means none)")
    parser.add_argument("--hit-part", type=int, choices=range(-1, 85), help="override synthetic fire hit part (-1 means none)")
    args = parser.parse_args()
    if (args.weapon_slot is not None or args.target_entity is not None or args.hit_part is not None) and args.kind not in ("fire", "server-fire"):
        parser.error("weapon/target/part overrides require fire or server-fire")
    data = DLL.read_bytes()
    if hashlib.sha256(data).hexdigest() != "a0bd4c43e6940e9738b9bb11a5d1bcb701edc48331256d6f05eadc4f9ab5f20f":
        raise ValueError("serializer offsets require the supplied September 2001 DLL")
    image = mapped_image(data)
    family = "mech" if args.kind == "commando" else args.kind
    # Every value is a synthetic serialization fixture, not a live setup.
    fields = {"title": {0x40: 1, 0x41: 1, 0x44: 1729, 0x48: 11,
                         0x4C: 22, 0x50: 33, 0x54: 44, 0x55: 2,
                         0x56: 3, 0x57: 4, 0x58: 55},
              "position": {0x30: 1, 0x34: -123, 0x38: 456, 0x3C: 77,
                            0x40: -11, 0x44: 22, 0x48: -33, 0x4C: 44},
              "client-position": {0x30: -123, 0x34: 456, 0x38: 77,
                                  0x3C: -11, 0x40: 22, 0x44: -33,
                                  0x48: 44, 0x4C: 0, 0x50: 0,
                                  0x54: 0, 0x58: 0},
              "fire": {0x30: 0, 0x31: 1, 0x34: 2, 0x38: -3, 0x3C: 4,
                       0x40: -5, 0x44: 6, 0x48: 7, 0x4C: -8,
                       0x50: 9, 0x54: -10, 0x58: 11},
              "server-fire": {0x30: 1, 0x31: 0, 0x32: 1, 0x34: 2,
                              0x38: -3, 0x3C: 4, 0x40: -5,
                              0x44: 6, 0x48: 7, 0x4C: -8,
                              0x50: 9, 0x54: -10},
              "damage": {0x30: 1, 0x31: 2, 0x32: 5, 0x33: 0, 0x34: 3},
              "local-damage": {0x30: 2, 0x31: 5, 0x32: 0, 0x33: 3},
              "mech": {0x30: 1, 0x31: 2, 0x64: -123, 0x68: 456,
                        0x6C: 3, 0x90: 17, 0x94: 18, 0x98: 19,
                        0x9C: 20, 0xA0: 4, 0xA1: 210, 0xA2: 5,
                        0xA3: 6}}.get(args.kind, {})
    if args.weapon_slot is not None:
        fields[0x30 if args.kind == "fire" else 0x31] = args.weapon_slot
    if args.target_entity is not None:
        fields[0x31 if args.kind == "fire" else 0x32] = args.target_entity + 1
    if args.hit_part is not None:
        fields[0x34] = args.hit_part + 1
    vectors = {}
    if args.kind == "commando":
        # Stock COM-4D initialization in FakeACat, 0x6905b696..0x6905b92b,
        # plus shared component initialization at 0x6905c096..0x6905c222.
        fields = {0x30: 0, 0x31: 1, 0x64: 10000, 0x68: 20000, 0x6C: 0,
                  0x94: 6, 0xA0: 5, 0xA1: 30, 0xA2: 12, 0xA3: 0}
        vectors = {0xA8: [[armor, loc, 0] for loc, armor in enumerate(
                          [8, 8, 10, 10, 15, 13, 13, 11, 8, 8, 27])],
                   0xBC: [[3, 1, 0], [0, 0, 0], [12, 4, 0]],
                   0xD0: [[index, 0, 0] for index in range(21)] +
                         [[24, 5, 0], [24, 5, 0], [24, 6, 0], [24, 6, 0],
                          [22, 2, 0], [22, 2, 0], [22, 3, 0], [22, 3, 0], [22, 7, 0]],
                   0xE4: [[11, 6, 0]]}
    byte_fields = ({0x40, 0x41, 0x54, 0x55, 0x56, 0x57, 0x58} if family == "title"
                   else {0x30} if family == "position"
                   else set() if family == "client-position"
                   else {0x30, 0x31, 0x58} if family == "fire"
                   else {0x30, 0x31, 0x32} if family == "server-fire"
                   else {0x30, 0x31, 0x32, 0x33, 0x34} if family in ("damage", "local-damage")
                   else {0x30, 0x31, 0x6C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4})
    setup = []
    for offset, value in fields.items():
        size = "BYTE" if offset in byte_fields else "DWORD"
        setup.append(f"mov {size} PTR [command+{offset}], {value}")
    strings = ({0x30: "Fixture"} if family == "title" else {} if family in ("ready", "position", "client-position", "fire", "server-fire", "damage", "local-damage") else {
        0x34: "FixturePilot", 0x44: "COM-4D", 0x54: "FixtureTeam",
        0x70: "FixtureModel", 0x80: "FixtureSkin"})
    if args.kind == "commando":
        strings = {0x34: "", 0x44: "FixturePilot", 0x54: "", 0x70: "COM-4D", 0x80: "Commando"}
    string_data = []
    if family not in ("ready", "position", "client-position", "fire", "server-fire", "damage", "local-damage"):
        for index, (offset, value) in enumerate(strings.items()):
            setup += [f"mov DWORD PTR [command+{offset + 4}], OFFSET string{index}",
                      f"mov DWORD PTR [command+{offset + 8}], {len(value)}"]
            string_data.append(f'string{index}: .asciz "{value}"')
    for index, (offset, records) in enumerate(vectors.items()):
        size = 3 * len(records)
        # VC6 vector has its allocator byte/padding before the three pointers.
        setup += [f"mov DWORD PTR [command+{offset + 4}], OFFSET vector{index}",
                  f"mov DWORD PTR [command+{offset + 8}], OFFSET vector{index}+{size}",
                  f"mov DWORD PTR [command+{offset + 12}], OFFSET vector{index}+{size}"]
        string_data.append(f"vector{index}: .byte " + ",".join(str(b) for record in records for b in record))
    writer = {"title": 0x69072BE0, "mech": 0x6906E830, "ready": 0x6906C600,
              "position": 0x6906F350, "client-position": 0x6906F700,
              "fire": 0x69070C20, "server-fire": 0x69070810,
              "damage": 0x6906BAA0, "local-damage": 0x6906BE00}[family]
    command_type = {"title": 0x48, "mech": 0x40, "ready": 0x3E,
                    "position": 0x41, "client-position": 0x06, "fire": 0x0A,
                    "server-fire": 0x44, "damage": 0x42, "local-damage": 0x43}[family]
    setup += [f"mov BYTE PTR [command+0x24], {command_type}",
              "push OFFSET stream", "lea ecx, [command+4]",
              f"mov eax, {writer}", "call eax"]
    asm = f""".intel_syntax noprefix
.section .text
.global _start
_start:
 mov DWORD PTR [0x6908b408], OFFSET move_bytes
 {chr(10).join(setup)}
 mov edx, [stream+8]
 mov [result], edx
 push edx
 push OFFSET buffer
 push OFFSET original
 call move_bytes
 add esp, 12
 # Original checksum instruction bytes, in both directions.
 push DWORD PTR [stream+8]
 push OFFSET buffer
 mov ecx, OFFSET context
 mov eax, 0x69075200
 call eax
 mov [result+4], eax
 mov BYTE PTR [context+4], 1
 push DWORD PTR [stream+8]
 push OFFSET buffer
 mov ecx, OFFSET context
 mov eax, 0x69075200
 call eax
 mov [result+8], eax
 # Native prefix, checksum, and ESC framing. Flag 1 selects server direction.
 mov ecx, OFFSET context
 mov eax, 0x69075080
 call eax
 mov ecx, OFFSET context
 mov eax, 0x690750f0
 call eax
 mov ecx, OFFSET context
 mov eax, 0x69075170
 call eax
 mov edx, [stream+8]
 mov [result+12], edx
 push edx
 push OFFSET buffer
 push OFFSET framed
 call move_bytes
 add esp, 12
 # The native receiver expects the terminal NUL to be included in its length.
 mov eax, [stream+8]
 mov BYTE PTR [buffer+eax], 0
 inc DWORD PTR [stream+8]
 mov DWORD PTR [stream+16], 0
 mov ecx, OFFSET context
 mov eax, 0x690751c0
 call eax
 mov ecx, OFFSET context
 mov eax, 0x69075110
 call eax
 movzx eax, al
 mov [result+16], eax
 mov ecx, OFFSET context
 mov eax, 0x690750b0
 call eax
 mov edx, [stream+8]
 mov [result+20], edx
 mov ecx, OFFSET result
 mov edx, 24
 call write_stdout
 mov ecx, OFFSET original
 mov edx, [result]
 call write_stdout
 mov ecx, OFFSET framed
 mov edx, [result+12]
 call write_stdout
 mov ecx, OFFSET buffer
 mov edx, [stream+8]
 call write_stdout
 mov eax, 1
 xor ebx, ebx
 int 0x80
move_bytes:
 push esi
 push edi
 mov edi, [esp+12]
 mov esi, [esp+16]
 mov ecx, [esp+20]
 mov eax, edi
 cmp edi, esi
 jbe copy_forward
 test ecx, ecx
 jz copy_done
 lea edi, [edi+ecx-1]
 lea esi, [esi+ecx-1]
 std
 rep movsb
 cld
 jmp copy_done
copy_forward:
 rep movsb
copy_done:
 pop edi
 pop esi
 ret
write_stdout:
 mov eax, 4
 mov ebx, 1
 int 0x80
 ret
raw_write:
 push esi
 push edi
 mov esi, [esp+12]
 mov edx, [esp+16]
 mov edi, [ecx+4]
 add edi, [ecx+16]
 add [ecx+16], edx
 cmp DWORD PTR [ecx+16], 4096
 ja failed
 mov eax, [ecx+16]
 mov [ecx+8], eax
 mov ecx, edx
 rep movsb
 pop edi
 pop esi
 ret 8
failed:
 mov eax, 1
 mov ebx, 2
 int 0x80
.section .data
stream: .long vtable, buffer, 0, 4096, 0, 0, 0
vtable: .long 0, 0, raw_write
context: .long stream, 0
result: .space 24
command: .space 256
{chr(10).join(string_data)}
original: .space 4096
framed: .space 4096
buffer: .space 4096
.section .peimage, "awx"
.incbin "image.bin"
"""
    linker = """PHDRS { code PT_LOAD FLAGS(5); data PT_LOAD FLAGS(6); image PT_LOAD FLAGS(7); }
SECTIONS {
 . = 0x08048000; .text : { *(.text) } :code
 . = ALIGN(4096); .data : { *(.data) } :data
 . = 0x69000000; .peimage : { *(.peimage) } :image
 /DISCARD/ : { *(.note*) *(.comment*) }
}
"""
    with tempfile.TemporaryDirectory(prefix="mpbt-combat-fixture-") as directory:
        path = Path(directory)
        (path / "image.bin").write_bytes(image)
        (path / "fixture.s").write_text(asm)
        (path / "fixture.ld").write_text(linker)
        subprocess.run(["as", "--32", "-o", "fixture.o", "fixture.s"], cwd=path, check=True)
        subprocess.run(["ld", "-melf_i386", "--no-warn-rwx-segments", "-T", "fixture.ld",
                        "-o", "fixture", "fixture.o"], cwd=path, check=True)
        output = subprocess.run([str(path / "fixture")], capture_output=True, check=True,
                                timeout=5).stdout
    length, client_checksum, server_checksum, frame_length, valid, decoded_length = struct.unpack_from("<6I", output)
    command_bytes = output[24:24 + length]
    frame_bytes = output[24 + length:24 + length + frame_length]
    decoded = output[24 + length + frame_length:]
    if length + frame_length + decoded_length != len(output) - 24:
        raise ValueError("incomplete native fixture")
    if valid != 1 or decoded != command_bytes:
        raise ValueError("native envelope round trip failed")
    print(json.dumps({"dll_sha256": hashlib.sha256(data).hexdigest(),
                      "kind": args.kind, "fields": {hex(k): v for k, v in fields.items()},
                      "strings": {hex(k): v for k, v in strings.items()},
                      "vectors": {hex(k): v for k, v in vectors.items()},
                      "command_hex": command_bytes.hex(),
                      "server_frame_hex": frame_bytes.hex(),
                      "native_decode_matches": True,
                      "client_checksum": client_checksum,
                      "server_checksum": server_checksum}, indent=2))


if __name__ == "__main__":
    main()
