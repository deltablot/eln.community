export class Partage {
  // AES with Galois/Counter Mode
  encryptionAlgo = 'AES-GCM';
  saltLength = 16;
  ivLength = 12;
  keyDerivationFunction = 'PBKDF2';
  keyDerivationHashAlgo = 'SHA-256';
  iterations = 100000;
  // number of bytes used to store the length of metadata
  headerLength = 2;

  constructor() {
    this.encoder = new TextEncoder();
    this.salt = crypto.getRandomValues(new Uint8Array(this.saltLength));
    this.iv = crypto.getRandomValues(new Uint8Array(this.ivLength));
  }

  getMetadata(file, text) {
    return {
      content_type: file.type,
      created_at: new Date().toISOString(),
      filename: file.name,
      size: file.size,
      text: text,
    };
  }

  async getEncryptedBlob(file, text, passphrase) {
    let arrayBuffer = new ArrayBuffer(0);
    if (file.size > 0) {
      arrayBuffer = await file.arrayBuffer();
    }
    // import passphrase as key for derivation
    const baseKey = await this.getBaseKey(passphrase);
    const derivedKey = await this.getDerivedKey(baseKey, ['encrypt']);
    // METADATA
    const metadata = this.getMetadata(file, text);
    const metadataEncoded = this.encoder.encode(JSON.stringify(metadata));
    // header will store the metadata length
    const header = new Uint8Array(this.headerLength);
    new DataView(header.buffer).setUint16(0, metadataEncoded.byteLength, false);

    // Encrypted data = header + metadata + file
    const combinedLength = header.byteLength + metadataEncoded.byteLength + arrayBuffer.byteLength;
    const taggedFile = new Uint8Array(combinedLength);
    taggedFile.set(header, 0);
    taggedFile.set(metadataEncoded, header.byteLength);
    taggedFile.set(new Uint8Array(arrayBuffer), header.byteLength + metadataEncoded.byteLength);

    const encryptedBuffer = await this.encrypt(derivedKey, taggedFile);

    const ciphertextArray = new Uint8Array(encryptedBuffer);
    // Allocate a new buffer: 16 bytes for salt, 12 bytes for IV, plus the ciphertext length.
    const combinedBuffer = new Uint8Array(this.saltLength + this.iv.byteLength + ciphertextArray.byteLength);
    // Copy the salt at the beginning.
    combinedBuffer.set(this.salt, 0);
    // Append the IV after the salt.
    combinedBuffer.set(this.iv, this.salt.byteLength);
    // Append the ciphertext after the salt and IV
    combinedBuffer.set(ciphertextArray, this.salt.byteLength + this.iv.byteLength);

    // Create a Blob from the combined buffer and append it to the FormData.
    return new Blob([combinedBuffer], { type: "application/octet-stream" });
  }

  async getBaseKey(passphrase) {
    const passphraseBuffer = this.encoder.encode(passphrase);
    return crypto.subtle.importKey(
      "raw",
      passphraseBuffer,
      this.keyDerivationFunction,
      false,
      ["deriveKey"]
    );
  }


  async getDerivedKey(baseKey, keyUsages, salt = null) {
      return crypto.subtle.deriveKey(
        {
          name: this.keyDerivationFunction,
          salt: salt ?? this.salt,
          iterations: this.iterations,
          hash: this.keyDerivationHashAlgo,
        },
        baseKey,
        { name: this.encryptionAlgo, length: 256 },
        false,
        keyUsages
      );
  }

  async encrypt(key, data) {
    return crypto.subtle.encrypt(
      { name: this.encryptionAlgo, iv: this.iv },
      key,
      data,
    );
  }

  async decrypt(derivedKey, iv, ciphertext) {
    return crypto.subtle.decrypt(
      { name: this.encryptionAlgo, iv: iv },
      derivedKey,
      ciphertext
    );
  }
}
