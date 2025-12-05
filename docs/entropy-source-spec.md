# EAMSA 512 Entropy Source Specification



## 1. Purpose and scope



This document describes the design, operation, and validation strategy for the entropy source used by EAMSA 512. The entropy source combines deterministic chaotic systems (Lorenz + hyperchaotic dynamics) with cryptographic conditioning (SHA3‑512) and NIST‑style statistical testing in order to achieve a target of ≥ 7.99 bits of entropy per byte for key generation and nonce derivation.



The design goal is to:



- Provide a high‑throughput, high‑quality entropy source suitable for cryptographic random bit generation.

- Allow analysis against NIST SP 800‑90B requirements for entropy sources used by random bit generators. 

- Integrate cleanly with the EAMSA 512 KDF and key‑agreement layer.



---



## 2. Design overview



### 2.1 Components



The entropy subsystem is composed of three conceptual layers:



1. *\*Chaos generator*\*  

&nbsp;  A deterministic but highly sensitive dynamical system (Lorenz + hyperchaotic extension) that produces raw samples exhibiting complex, noise‑like behavior with positive Lyapunov exponents.



2. *\*Sampling and digitization*\*  

&nbsp;  A deterministic procedure that periodically samples the state variables of the chaotic system, quantizes them to fixed‑precision integers, and assembles them into raw bitstrings.



3. *\*Conditioning and extraction*\*  

&nbsp;  A cryptographic conditioner based on SHA3‑512 that maps raw, partially unpredictable inputs to uniformly distributed 512‑bit outputs, in line with the “conditioning component” model from NIST SP 800‑90B. 



The resulting conditioned outputs feed into:



- The DRBG and KDF for key generation and key expansion.

- Session nonces and protocol‑level randomness requirements.



### 2.2 Entropy roles



EAMSA 512 uses its entropy source for:



- Seeding and reseeding of internal DRBGs.

- Generating nonces and salts for key establishment.

- Supplementing entropy for key‑agreement inputs and `OtherInfo` in the KDF.



It is *\*not*\* used to replace approved cryptographic primitives; instead, it augments them as a noise source in front of SHA3‑512 and standardized KDF constructions.



---



## 3. Chaos generator



### 3.1 Dynamical systems



The core chaotic system is formed by combining:



- A Lorenz system (three differential equations with classical chaotic parameter sets).

- One or more additional state variables forming a hyperchaotic extension to increase the number of positive Lyapunov exponents and complexity of the attractor.



The system is integrated numerically with a small, fixed time step using a stable integration scheme. Parameters are chosen in ranges known to yield strong chaotic behavior (positive largest Lyapunov exponent, non‑periodic trajectories).



### 3.2 Initialization



Each instance of the generator is initialized with:



- A seed derived from system startup conditions (time, hardware features, HSM‑provided randomness, or OS RNG).

- Distinct initial conditions for each state variable within pre‑defined safe ranges.



Initialization is combined with external entropy through SHA3‑512 before setting the actual internal state, in order to decorrelate the chaos seed from direct external observables.



### 3.3 Evolution and mixing



On each step:



1. The chaotic system is advanced by one or more integration steps.

2. Selected state variables (e.g., x, y, z, and one or more hyperchaotic coordinates) are sampled.

3. The samples are scaled and quantized into fixed‑width integers (e.g., 16 or 32 bits per coordinate), then concatenated into a raw word.



These raw words form the *\*noise samples*\* that feed the conditioning component.



---



## 4. Sampling and digitization



### 4.1 Sampling schedule



To avoid aliasing and short‑term correlations:



- Sampling occurs at intervals large enough (in integration steps) to decorrelate consecutive samples but small enough to maintain throughput.

- Empirical analysis (autocorrelation and approximate entropy) is used to select sampling period and spacing, following approaches similar to those used in chaotic RNG evaluations.



### 4.2 Quantization



Each sampled state variable is:



1. Normalized to a bounded interval based on expected attractor range.

2. Quantized to a fixed integer range, e.g., mapping to a 16‑bit signed or unsigned integer.

3. Packed into bytes in a deterministic order.



Multiple variables are concatenated, e.g.:



- `sample = Q(x) || Q(y) || Q(z) || Q(w1) || Q(w2) …`



The resulting sample length and bit ordering are fixed and documented, which is important for SP 800‑90B entropy estimation tooling.



### 4.3 Noise source characterization



Static analysis and simulations are used to:



- Confirm that the power spectrum is broad and noise‑like.

- Verify sensitivity to initial conditions via Lyapunov exponent estimation.

- Compare correlation structure and approximate entropy with white noise sources.



---



## 5. Conditioning with SHA3‑512



### 5.1 Rationale



NIST SP 800‑90B distinguishes between the *\*noise source*\* and an optional *\*conditioning component*\* that increases the effective min‑entropy per output symbol via an approved cryptographic primitive.



EAMSA 512 uses SHA3‑512 as a conditioning component to:



- Whiten the output of the chaotic noise source.

- Aggregate multiple noise samples into a single 512‑bit output with high min‑entropy.

- Provide a conservative bound on entropy per bit by relying on the preimage and collision resistance of SHA3‑512.



### 5.2 Construction



Let `R` be a buffer of raw samples from the chaos generator of fixed length (e.g., several kilobytes). The conditioner computes:



\\[

C = \\text{SHA3-512}( R )

\\]



Properties:



- `C` is a 512‑bit string used as a *\*conditioned entropy output*\*.

- Multiple such outputs \(C\_1, C\_2, …\) are generated by moving the sampling window forward and repeating the process.

- The internal state of SHA3‑512 is not reused across unrelated streams unless deliberately configured (e.g., via XOF‑like usage) to avoid cross‑contamination.



---



## 6. Entropy estimation and validation



### 6.1 NIST SP 800‑90B framework



Entropy validation for this design is aligned conceptually with NIST SP 800‑90B:



- The chaotic dynamics + digitization form the \*\*noise source\*\*.

- SHA3‑512 acts as the \*\*conditioning component\*\*.

- Statistical testing and estimation are applied to raw samples as required by 90B.

- Min‑entropy bounds are derived and used to justify the claimed entropy rate.



### 6.2 Raw noise testing



On the raw (pre‑conditioning) samples:



- *\*Autocorrelation analysis*\* to detect short‑range dependencies.

- *\*Frequency and runs tests*\* to check bias and basic structure.

- *\*Approximate entropy and related measures*\* to compare complexity to white noise.

- Additional tests from established suites (e.g., NIST SP 800‑22, Dieharder, TestU01) as engineering checks.



These tests provide evidence that the sampled chaotic system behaves comparably to high‑quality noise sources when correctly parameterized and sampled.



### 6.3 Entropy estimation



Following the methodology outlined for 90B:


- Collect large datasets of raw samples (before conditioning).

- Apply both IID and non‑IID estimators where appropriate.

- Compute conservative min‑entropy estimates per sample (per byte or per symbol).

- Take the lowest min‑entropy estimate across all applicable estimators as the claimed entropy rate, in line with 90B’s “lowest‑wins” principle.



If a formal 90B validation is pursued, data would be submitted to an accredited lab and the Entropy Source Validation (ESV) test system.



### 6.4 Conditioned output analysis



Although SHA3‑512 conditioning is designed to significantly increase effective min‑entropy per output bit, EAMSA 512 maintains a conservative posture:



- Claims of ≥ 7.99 bits/byte are based on a combination of:

&nbsp; - 90B‑style raw noise estimates.

&nbsp; - The assumption that SHA3‑512, as an approved cryptographic hash, behaves as a good extractor.

- Additional statistical tests are applied to `C` outputs to confirm the absence of detectable structure.



---



## 7. Health tests and runtime monitoring



### 7.1 Health tests



NIST SP 800‑90B mandates ongoing health tests to detect catastrophic failures in entropy sources.

EAMSA 512 includes:


- *\*Repetition count test*\* on raw samples to detect stuck or repeated outputs.

- *\*Adaptive proportion test*\* on bitstreams to detect bias shifts.

- Optional additional “sanity checks” (compression and collision‑style checks) on sliding windows of data.



If any health test fails:



- The entropy source is flagged as unhealthy.

- The random‑bit generation service enters an error state.

- Keys and nonces are not generated until the fault is cleared or the module is restarted.



### 7.2 Logging and alerts



- All health‑test failures and entropy source errors are logged.

- When integrated with HSMs or external monitoring, alerts can be raised to operations staff.

- Logs are carefully scrubbed to avoid leaking internal state or raw entropy samples.



---



## 8. Integration with DRBG and KDF. 


### 8.1 DRBG seeding and reseeding



Conditioned outputs \\( C\_i \\) feed:



- The initial seed of an internal DRBG or directly into the KDF when deriving keys.

- Periodic reseeding after a configured number of bytes or time interval, as recommended for robust random bit generators.



### 8.2 Key establishment



In the key‑agreement layer:



- `chaos\_output` (one or more \( C\_i \)) is incorporated into `OtherInfo` and/or combined with the base secret before KDF input.

- This provides extra randomness tied to the current environment and time, making derived keys more resistant to partial exposure of long‑term secrets.



---



## 9. Operational guidance



### 9.1 Deployment considerations



- Ensure the module has sufficient CPU resources; the chaos generator and SHA3‑512 conditioning add modest overhead but are designed to remain efficient.

- On virtualized or cloud environments, avoid deterministic seeding sources that may reduce true diversity across instances.

- When used with hardware noise sources (e.g., HSM TRNGs), combine external entropy with the chaotic generator rather than replacing one with the other. 



### 9.2 Configuration



- Expose parameters such as sampling rate, chaos system parameters, and reseed intervals via configuration with safe defaults and bounds.

- Lock down configuration changes in production (RBAC) to prevent weakening the entropy source inadvertently.



### 9.3 Validation and audits



- Maintain records of entropy testing campaigns and min‑entropy estimates.

- For regulated deployments, align documentation with 90B and CMVP entropy validation guidelines.



---



## 10. Limitations and future work



- Chaotic deterministic systems, by themselves, are not a substitute for approved cryptographic primitives; their outputs are always conditioned through SHA3‑512 in this design.

- Future work may incorporate:

&nbsp; - Additional physical or hardware noise sources.

&nbsp; - AI‑based techniques to automatically tune chaos system parameters while maintaining predictable security margins, as explored in recent research on AI‑enhanced chaotic key generation.\[web:220]\[web:223]

&nbsp; - Formal 90B validation and publication of entropy certificates.



---



By combining a rigorously analyzed chaotic noise source with SHA3‑512 conditioning and NIST‑style health tests and entropy estimation, the EAMSA 512 entropy subsystem is engineered to provide a robust and auditable foundation for all cryptographic key material.




