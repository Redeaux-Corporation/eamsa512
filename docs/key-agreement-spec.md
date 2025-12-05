# EAMSA 512 Key Agreement and Derivation Specification



## 1. Purpose and scope



This document specifies the key-agreement and key-derivation scheme used by EAMSA 512. It combines a high-entropy chaos-based source with a NIST-style concatenation KDF based on SHA3‑512 to derive a structured set of working keys for encryption and authentication.



The goals are:

- To ensure strong, well-understood security properties aligned with NIST SP 800‑56A/56C recommendations for key-establishment and key-derivation methods.

- To provide clear separation of roles for derived keys (encryption vs. authentication vs. internal subkeys).

- To allow formal analysis and compliance assessment.



---



## 2. High-level design



At a high level, EAMSA 512 key agreement and derivation proceed as follows:



1. Establish or provision an initial secret (Z), which serves as the shared secret input to the KDF.

2. Collect additional entropy from the internal chaos-based generator (Lorenz + hyperchaotic system) and fold it into the KDF context.

3. Derive a fixed number of 128‑bit subkeys using a SHA3‑512 concatenation KDF, following the structure recommended for KDFs in key-establishment schemes.

4. Assign each derived subkey to a specific role in the EAMSA 512 pipeline (round keys, whitening keys, HMAC key, etc.).



This design separates the *agreement* of an initial secret from the *derivation* of multiple internal keys, which is consistent with NIST guidance for pair‑wise key-establishment and subsequent key-derivation.



---



## 3. Inputs and notation



### 3.1 Inputs



The KDF takes the following inputs:



- (Z): Shared secret bit string (at least 256 bits).

- `nonce`: A 128‑bit per‑session value, unique for each key‑establishment event.

- `chaos\_seed`: Initial state and parameters for the chaos-based generator.

- `chaos\_output`: A fixed-length bitstring output from the chaos generator, post‑whitening.

- `OtherInfo`: Context data as described below (algorithm identifiers, key lengths, labels).



### 3.2 OtherInfo structure



`OtherInfo` is constructed as a concatenation of typed fields in the style of SP 800‑56A Section 5.8.1:

- `AlgoID`: Identifier for “EAMSA512-ENC+HMAC-SHA3-512”.

- `KeyDataLen`: Total length of derived keying material in bits.

- `PartyUInfo`: Optional identifier for initiator.

- `PartyVInfo`: Optional identifier for responder.

- `SuppPubInfo`: Public context, including:

&nbsp; - Protocol version.

&nbsp; - Cipher suite identifier.

&nbsp; - Mode flags.

- `SuppPrivInfo`: Private context, including:

&nbsp; - Encoded `nonce`.

&nbsp; - Encoded `chaos\_output` (or its hash).



All fields are length‑delimited or encoded in a way that is unambiguous and deterministic.



---



## 4. Chaos-based entropy contribution



### 4.1 Chaos generator overview

The chaos generator uses a combination of Lorenz and hyperchaotic systems parameterized to ensure positive Lyapunov exponents, which indicate sensitivity to initial conditions and exponential divergence.

Key characteristics:

- Internal state dimension ≥ 3 (Lorenz) plus additional state variables for hyperchaos.

- Floating‑point or fixed‑point integration with small step size.

- Periodic sampling of state coordinates to form raw bit sequences.



### 4.2 Whitening and compression



To avoid relying on the raw dynamics for cryptographic security:



1. The raw chaotic samples are quantized and concatenated into a bitstring.

2. The bitstring is passed through SHA3‑512 to provide diffusion and whitening.

3. The resulting 512‑bit hash is used as `chaos_output` and/or as part of `SuppPrivInfo` in the KDF.



This design turns the chaotic source into a high‑entropy auxiliary input rather than the primary cryptographic primitive, which aligns with NIST guidance that recommends using approved hash functions and KDFs for key derivation.



---



## 5. KDF construction



### 5.1 Function



EAMSA 512 uses a SHA3‑512‑based concatenation KDF similar to the “KDF in counter mode” in SP 800‑56A/56C, adapted to SHA3‑512 as the underlying hash.

Let:

- ( H ) be SHA3‑512.

- `KDF(Z, OtherInfo, L)` output `L` bits of keying material.



The KDF is defined as:



1. Let ( n = lceil L / 512 rceil ).

2. For counter (i = 1, 2, …, n), compute:

&nbsp;  [

&nbsp;  K_i = H( text{I2OSP}(i, 4) parallel Z parallel OtherInfo )

&nbsp;  ]

&nbsp;  where `I2OSP(i, 4)` encodes the 32‑bit counter as 4 bytes big‑endian.

3. Concatenate (K_1 parallel K_2 parallel … parallel K_n), and take the leftmost (L) bits as the output.



### 5.2 Intended output length



EAMSA 512 derives 11 independent 128‑bit keys (total 1408 bits) from the KDF:


- (L = 1408) bits.

- (n = lceil 1408 / 512 rceil = 3) blocks of SHA3‑512 output.

The total KDF output is 1536 bits; the final 128 bits are discarded to yield exactly 1408 bits of keying material.



---



## 6. Derived key structure and roles



The 1408‑bit KDF output is partitioned into 11 contiguous 128‑bit keys:



- `K[0]`–`K[7]`: 8 × 128‑bit keys used as round or mixing keys in the 512‑bit encryption core.

- `K[8]`: 128‑bit whitening or tweak key (e.g., pre/post‑processing of the state).

- `K[9]`: 128‑bit key for internal integrity or auxiliary operations.

- `K[10]`: 128‑bit base used to expand into an HMAC‑SHA3‑512 key (e.g., via HKDF‑like expansion) for authenticated encryption.



Each key has a fixed role; keys are never reused across roles or protocols. This enforces *key separation*, a common best practice and also consistent with NIST guidance on compartmentalizing derived keys by usage.



---



## 7. Session establishment and lifecycle



### 7.1 Session setup



A typical session proceeds as follows:



1. *\*Input collection*\*

&nbsp;  - Obtain or negotiate (Z) (e.g., provisioned master secret or result of an external key-agreement mechanism).

&nbsp;  - Generate a fresh 128‑bit `nonce` for the session.

&nbsp;  - Run the chaos generator for a defined number of steps and compute `chaos_output` via SHA3‑512.



2. *\*Construct OtherInfo*\*

&nbsp;  - Encode protocol version, cipher suite, roles, and usage labels.

&nbsp;  - Encode `nonce` and `chaos_output` into `SuppPrivInfo`.



3. *\*Derive keys*\*

&nbsp;  - Invoke the KDF with (Z) and `OtherInfo` to produce 11 × 128‑bit keys.


4. *\*Initialize cipher state*\*

&nbsp;  - Configure the 512‑bit block core with the appropriate subkeys.

&nbsp;  - Configure HMAC layer with expanded key material from `K[10]`.



### 7.2 Key lifetime



- All 11 keys are *\*session‑scoped*\*: they MUST NOT be reused across sessions with different nonces.

- Sessions are bounded in time and data volume; beyond thresholds, a new `nonce` and fresh derivation are required.

- Long‑term secrets such as (Z) and HSM‑protected master keys have their own rotation policies (e.g., annually or by policy).



---



## 8. Security properties



### 8.1 Forward and backward secrecy



- If (Z) is established by a secure external key‑agreement scheme (e.g., ECDH in an HSM or external KMS), compromise of a single session’s derived keys does not reveal past or future session keys, assuming `nonce` and chaos outputs are unique and the KDF remains secure.

- Compromise of derived keys does not reveal (Z) due to the preimage resistance of SHA3‑512.



### 8.2 Key separation and misuse resistance



- Separate roles for derived keys (encryption vs. MAC vs. internal) help mitigate cross‑protocol and cross‑use attacks.

- `OtherInfo` encodes algorithm identifiers and usage context, which helps prevent key‑material reuse across different algorithms or configurations.



### 8.3 Entropy robustness



- Even if the chaos source were degraded, the security of the scheme falls back to that of the underlying hash and KDF with respect to (Z) and `nonce`, although effective entropy may be lower.

- Continuous tests on the entropy source can be used to detect severe degradation and force the module into an error state.



---



## 9. Compliance considerations



### 9.1 Alignment with NIST SP 800‑56A / 56C



- The construction follows the counter‑based concatenation KDF pattern described in SP 800‑56A and further refined in SP 800‑56C, with adaptation to SHA3‑512 as the hash function.

- The separation between key agreement (provision or establishment of (Z)) and key derivation (expansion into working keys) matches the logical split in NIST’s key‑establishment framework.



### 9.2 FIPS 140‑2 environment



- When deployed with a FIPS‑validated HSM, (Z) and long‑term secrets SHOULD be generated and stored within the HSM boundary, and only derived session keys (or wrapped forms) are exposed to EAMSA 512.

- `OtherInfo` and `nonce` handling follows best practices for ensuring that derived keys have well‑defined security strengths and contexts.



---



## 10. Implementation notes



- The KDF implementation must be constant‑time with respect to secret values, avoiding data‑dependent branching on (Z) or `OtherInfo`.

- All internal buffers that hold (Z), `nonce`, chaos seeds, `chaos\_output`, and derived keys must be securely zeroized when no longer needed.

- Logging MUST NOT include any of the above secret values or raw KDF inputs/outputs.



---



## 11. Future extensions

The key-agreement layer is designed to be algorithm‑agile:

- (Z) may in future be produced by different approved key‑establishment schemes (e.g., post‑quantum KEMs) without changing the overall KDF interface.

- The KDF may be re‑parameterized to use alternate approved hashes or HKDF‑like constructions if standards evolve, as long as key roles and context encoding remain consistent.




