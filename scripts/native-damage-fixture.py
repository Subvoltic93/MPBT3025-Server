#!/usr/bin/env python3
"""Execute native armor resolution and armor assignment in an isolated i386 process.

No DLL initialization, Wine, client injection, or network access. The original
armor resolver/GetDamageState run on synthetic arrays; AddDamage is replaced
with a bounded recorder. Original Damage runs with a synthetic Reality getter
and non-local Mech, avoiding rendering/server callbacks. No destruction cases.
"""
import hashlib
import json
from pathlib import Path
import runpy
import struct
import subprocess
import tempfile


def main():
    native = runpy.run_path(str(Path(__file__).with_name('native-combat-fixture.py')))
    data = native['DLL'].read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    if digest != 'a0bd4c43e6940e9738b9bb11a5d1bcb701edc48331256d6f05eadc4f9ab5f20f':
        raise ValueError('requires the pinned September 2001 BTCombat DLL')
    image = native['mapped_image'](data)
    # Input order: armor remaining, incoming damage. Armor index 4, credit 7.
    cases = [(15, 5), (3, 5)]
    calls = []
    for i, (armor, damage) in enumerate(cases):
        calls.append(f'''
 call reset_mech
 mov DWORD PTR [mech+0x1a0], {armor}
 mov DWORD PTR [active_result], OFFSET results+{i * 52}
 push 7
 push 1
 push 4
 push {damage}
 push OFFSET mech
 mov eax, 0x690500c0
 call eax
 add esp, 20
''')
    asm = f'''.intel_syntax noprefix
.section .text
.global _start
_start:
 # Replace AddDamage with a bounded recorder; preserve original resolver.
 mov BYTE PTR [0x6904a450], 0xe9
 mov eax, OFFSET record_damage
 sub eax, 0x6904a455
 mov DWORD PTR [0x6904a451], eax
 # Damage only needs Reality's mode flag on these non-destructive armor paths.
 mov BYTE PTR [0x690560e0], 0xb8
 mov DWORD PTR [0x690560e1], OFFSET reality
 mov BYTE PTR [0x690560e5], 0xc3
 {''.join(calls)}
 # Original Damage uses absolute remaining armor and never increases it.
 call reset_mech
 push 7
 push 10
 push 25
 mov ecx, OFFSET mech
 mov eax, 0x69048100
 call eax
 mov eax, DWORD PTR [mech+0x1a0]
 mov DWORD PTR [assignments], eax
 push 7
 push 10
 push 25
 mov ecx, OFFSET mech
 mov eax, 0x69048100
 call eax
 mov eax, DWORD PTR [mech+0x1a0]
 mov DWORD PTR [assignments+4], eax
 push 7
 push 12
 push 25
 mov ecx, OFFSET mech
 mov eax, 0x69048100
 call eax
 mov eax, DWORD PTR [mech+0x1a0]
 mov DWORD PTR [assignments+8], eax
 mov eax, 4
 mov ebx, 1
 mov ecx, OFFSET results
 mov edx, 116
 int 0x80
 mov eax, 1
 xor ebx, ebx
 int 0x80
reset_mech:
 xor edx, edx
reset_armor:
 mov DWORD PTR [mech+0x190+edx*4], 15
 mov DWORD PTR [mech+0x1bc+edx*4], 15
 inc edx
 cmp edx, 11
 jb reset_armor
 xor edx, edx
reset_internal:
 mov DWORD PTR [mech+0x1e8+edx*4], 20
 inc edx
 cmp edx, 8
 jb reset_internal
 ret
record_damage:
 mov edx, [active_result]
 mov eax, [edx]
 cmp eax, 4
 jae failed
 imul eax, 12
 lea edx, [edx+eax+4]
 mov eax, [esp+4]
 mov [edx], eax
 mov eax, [esp+8]
 mov [edx+4], eax
 movzx eax, WORD PTR [esp+12]
 mov [edx+8], eax
 mov edx, [active_result]
 inc DWORD PTR [edx]
 ret 12
failed:
 mov eax, 1
 mov ebx, 2
 int 0x80
.section .data
active_result: .long 0
results: .space 104
assignments: .space 12
mech: .space 2048
reality: .space 512
.section .peimage, "awx"
.incbin "image.bin"
'''
    linker = '''PHDRS { code PT_LOAD FLAGS(5); data PT_LOAD FLAGS(6); image PT_LOAD FLAGS(7); }
SECTIONS {
 . = 0x08048000; .text : { *(.text) } :code
 . = ALIGN(4096); .data : { *(.data) } :data
 . = 0x69000000; .peimage : { *(.peimage) } :image
 /DISCARD/ : { *(.note*) *(.comment*) }
}
'''
    with tempfile.TemporaryDirectory(prefix='mpbt-damage-fixture-') as directory:
        path = Path(directory)
        (path / 'image.bin').write_bytes(image)
        (path / 'fixture.s').write_text(asm)
        (path / 'fixture.ld').write_text(linker)
        subprocess.run(['as', '--32', '-o', 'fixture.o', 'fixture.s'], cwd=path, check=True)
        subprocess.run(['ld', '-melf_i386', '--no-warn-rwx-segments', '-T', 'fixture.ld',
                        '-o', 'fixture', 'fixture.o'], cwd=path, check=True)
        output = subprocess.run([str(path / 'fixture')], cwd=path, check=True,
                                capture_output=True, timeout=5).stdout
    if len(output) != 116:
        raise ValueError('incomplete native result')
    resolved = []
    for i, (armor, damage) in enumerate(cases):
        count = struct.unpack_from('<I', output, i * 52)[0]
        if count > 4:
            raise ValueError('native damage recorder exceeded bound')
        records = [list(struct.unpack_from('<3I', output, i * 52 + 4 + j * 12)) for j in range(count)]
        resolved.append({'armor_index': 4, 'armor_before': armor, 'internal_before': 20,
                         'incoming_damage': damage, 'credit_entity': 7, 'queued_records': records})
    print(json.dumps({'dll_sha256': digest, 'kind': 'armor-resolution',
                      'resolver': '0x690500c0', 'assignment': '0x69048100',
                      'substitutions': ['AddDamage bounded recorder', 'synthetic Reality getter'],
                      'cases': resolved,
                      'absolute_assignments': list(struct.unpack_from('<3I', output, 104))}, indent=2))


if __name__ == '__main__':
    main()
