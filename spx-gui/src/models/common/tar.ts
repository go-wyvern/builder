/**
 * @file tar.ts
 * @desc Utilities for working with tar files
 */

import { type Files, File } from './file'
import type { Metadata } from '../project'

const METADATA_FILE = 'metadata.json'

/**
 * Save project files and metadata to a tar file
 * @param metadata Project metadata
 * @param files Project files
 * @returns A File object containing the tar file
 */
export async function save(metadata: Metadata, files: Files): Promise<globalThis.File> {
  // We're using the tar-js library for browser-based tar creation
  const tarball = new Tar()

  // Add metadata file
  const metadataContent = JSON.stringify(metadata, null, 2)
  tarball.append(METADATA_FILE, new TextEncoder().encode(metadataContent))

  // Add all project files
  await Promise.all(
    Object.entries(files).map(async ([path, file]) => {
      if (!file) return
      const buffer = await file.arrayBuffer()
      tarball.append(path, new Uint8Array(buffer))
    })
  )

  // Generate the tar file
  const tarBuffer = tarball.out()

  return new globalThis.File([tarBuffer], `${metadata.name || 'project'}.tar`, { type: 'application/x-tar' })
}

/**
 * Load project files and metadata from a tar file
 * @param tarFile Tar file to load
 * @returns Object containing project metadata and files
 */
export async function load(tarFile: globalThis.File) {
  const buffer = await tarFile.arrayBuffer()
  const tarData = new Uint8Array(buffer)
  const metadata: Metadata = {}
  const files: Files = {}
  
  let offset = 0
  
  while (offset + 512 <= tarData.length) {

    const header = tarData.slice(offset, offset + 512)
    offset += 512
    

    if (isZeroBlock(header)) {
      break
    }
 
    let filenameLength = 0
    for (let i = 0; i < 100; i++) {
      if (header[i] === 0) {
        filenameLength = i
        break
      }
    }
    
    const path = new TextDecoder().decode(header.slice(0, filenameLength))
    const sizeStr = new TextDecoder().decode(header.slice(124, 135)).trim()
    const size = parseInt(sizeStr, 8)
    const content = tarData.slice(offset, offset + size)
    
    if (path === METADATA_FILE) {
      const text = new TextDecoder().decode(content)
      try {
        Object.assign(metadata, JSON.parse(text))
      } catch (e) {
        console.error('Failed to parse metadata:', e)
      }
    } else {
      const blob = new Blob([content])
      const fileName = path.split('/').pop() || path
      const customFile = new File(fileName, async () => {
        return content;
      }, { type: blob.type });
      
      files[path] = customFile;
    }
    
    offset += size
    const padding = 512 - (size % 512 || 512)
    if (padding < 512) {
      offset += padding
    }
  }
  
  return { metadata, files }
}

/**
 * Check if a block of data is a zero block
 * @param block The block of data to check
 * @returns True if the block is a zero block, false otherwise
 */
function isZeroBlock(block: Uint8Array): boolean {
  for (let i = 0; i < block.length; i++) {
    if (block[i] !== 0) {
      return false
    }
  }
  return true
}

/**
 * Mini tar implementation for the browser
 * Inspired by tar-js but simplified for our needs
 */
class Tar {
  private buffer: Uint8Array[] = []

  /**
   * Append a file to the tar archive
   * @param filename Name of the file
   * @param data Content of the file as Uint8Array
   */
  append(filename: string, data: Uint8Array): void {
    // Create header
    const header = new Uint8Array(512)

    // Filename (100 bytes)
    const filenameBytes = new TextEncoder().encode(filename)
    header.set(filenameBytes.slice(0, 100), 0)

    // File mode (8 bytes) - default to 644 octal
    const mode = new TextEncoder().encode('000644 ')
    header.set(mode, 100)

    // UID/GID (8 bytes each) - default to 0
    const uid = new TextEncoder().encode('000000 ')
    const gid = new TextEncoder().encode('000000 ')
    header.set(uid, 108)
    header.set(gid, 116)

    // File size (12 bytes) - octal
    const size = data.length.toString(8).padStart(11, '0') + ' '
    header.set(new TextEncoder().encode(size), 124)

    // Modification time (12 bytes) - unix timestamp in octal
    const mtime =
      Math.floor(Date.now() / 1000)
        .toString(8)
        .padStart(11, '0') + ' '
    header.set(new TextEncoder().encode(mtime), 136)

    // Checksum placeholder (8 bytes)
    const checksumPlaceholder = new TextEncoder().encode('        ')
    header.set(checksumPlaceholder, 148)

    // Type flag (1 byte) - '0' for regular file
    header.set(new TextEncoder().encode('0'), 156)

    // Calculate checksum
    let checksum = 0
    for (let i = 0; i < 512; i++) {
      checksum += header[i]
    }

    // Write checksum
    const checksumStr = checksum.toString(8).padStart(6, '0') + '\0 '
    header.set(new TextEncoder().encode(checksumStr), 148)

    // Add header and data to buffer
    this.buffer.push(header)
    this.buffer.push(data)

    // Pad data to a multiple of 512 bytes
    const padLength = 512 - (data.length % 512 || 512)
    if (padLength < 512) {
      this.buffer.push(new Uint8Array(padLength))
    }
  }

  /**
   * Get the tar file as a Uint8Array
   */
  out(): Uint8Array {
    // Add two empty blocks at the end
    this.buffer.push(new Uint8Array(1024))

    // Calculate total size
    const totalSize = this.buffer.reduce((size, arr) => size + arr.length, 0)

    // Create result buffer
    const result = new Uint8Array(totalSize)

    // Copy all buffers
    let offset = 0
    for (const arr of this.buffer) {
      result.set(arr, offset)
      offset += arr.length
    }

    return result
  }
}
