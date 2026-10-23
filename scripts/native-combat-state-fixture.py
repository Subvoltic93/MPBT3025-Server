#!/usr/bin/env python3
"""Execute native pose application and COM-4D internal initialization offline.

No Wine or networking. Reality/GetMech and SetPosition are bounded substitutes;
the original peer visitor and internal initialization instructions execute.
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
        raise ValueError('requires pinned BTCombat DLL')
    image = native['mapped_image'](data)
    asm = r'''.intel_syntax noprefix
.section .text
.global _start
_start:
 mov BYTE PTR [0x690560e0], 0xb8
 mov DWORD PTR [0x690560e1], OFFSET reality
 mov BYTE PTR [0x690560e5], 0xc3
 mov BYTE PTR [0x69056080], 0xe9
 mov eax, OFFSET get_mech
 sub eax, 0x69056085
 mov DWORD PTR [0x69056081], eax
 mov BYTE PTR [0x69045920], 0xe9
 mov eax, OFFSET record_position
 sub eax, 0x69045925
 mov DWORD PTR [0x69045921], eax
 fld QWORD PTR [pi]
 fstp QWORD PTR [0x690d4ac8]
 mov DWORD PTR [reality+0x140], 3
 mov BYTE PTR [command+0x30], 1
 mov DWORD PTR [command+0x34], 12345
 mov DWORD PTR [command+0x38], -54321
 mov DWORD PTR [command+0x3c], 13287
 push OFFSET command
 mov eax, 0x69073640
 call eax
 # Run exactly the original eight-region internal initialization loop.
 mov BYTE PTR [0x6904138a], 0xc3
 mov ebx, OFFSET mech
 mov DWORD PTR [mech+0x238], 30
 xor ecx, ecx
 mov esi, 9
 mov eax, 0x6904129b
 call eax
 mov esi, OFFSET mech+0x1e8
 mov edi, OFFSET internals
 mov ecx, 8
 rep movsd
 xor ebx, ebx
map_region:
 push ebx
 mov ecx, OFFSET mech
 mov eax, 0x69004010
 call eax
 mov DWORD PTR [armor_map+ebx*4], eax
 inc ebx
 cmp ebx, 11
 jb map_region
 mov eax, 4
 mov ebx, 1
 mov ecx, OFFSET results
 mov edx, 132
 int 0x80
 mov eax, 1
 xor ebx, ebx
 int 0x80
get_mech:
 mov eax, [esp+4]
 mov [entity], eax
 mov eax, OFFSET mech
 ret 4
record_position:
 push esi
 push edi
 lea esi, [esp+12]
 mov edi, OFFSET position
 mov ecx, 13
 rep movsd
 pop edi
 pop esi
 ret 52
.section .data
pi: .double 3.141592653589793
results:
entity: .long 0
position: .space 52
internals: .space 32
armor_map: .space 44
command: .space 80
reality: .space 512
mech: .space 2048
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
    with tempfile.TemporaryDirectory(prefix='mpbt-state-fixture-') as directory:
        path = Path(directory)
        (path/'image.bin').write_bytes(image)
        (path/'fixture.s').write_text(asm)
        (path/'fixture.ld').write_text(linker)
        subprocess.run(['as','--32','-o','fixture.o','fixture.s'],cwd=path,check=True)
        subprocess.run(['ld','-melf_i386','--no-warn-rwx-segments','-T','fixture.ld',
                        '-o','fixture','fixture.o'],cwd=path,check=True)
        output = subprocess.run([str(path/'fixture')],capture_output=True,check=True,timeout=5).stdout
    if len(output) != 132:
        raise ValueError('incomplete native state fixture')
    print(json.dumps({
        'dll_sha256': digest,
        'substitutions': ['synthetic Reality/GetMech','SetPosition argument recorder',
                          'return at end of internal initialization loop'],
        'peer_position': {'visitor': '0x69073640', 'set_position': '0x69045920',
                          'wire_x':12345,'wire_y':-54321,'wire_height':13287,
                          'entity':struct.unpack_from('<I',output,0)[0],
                          'world_xyz':list(struct.unpack_from('<3d',output,4))},
        'commando_internal': list(struct.unpack_from('<8I',output,56)),
        'armor_to_internal': list(struct.unpack_from('<11I',output,88)),
        'medium_laser': {'damage':struct.unpack_from('<I',image,0xb99b4+3*140)[0],
                         'max_range':struct.unpack_from('<I',image,0xb99dc+3*140)[0],
                         'recharge': (struct.unpack_from('<I',image,0xb99c4+3*140)[0]+1)*
                                     struct.unpack_from('<d',image,0x8dc48)[0],
                         'recharge_source':'Init +0x28; FireWeapon sets time + recharge; IsReady checks deadline'}
    },indent=2))


if __name__ == '__main__':
    main()
