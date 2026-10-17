package unattend

import (
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"strconv"
	"strings"
)

// Alfabeto base64 propio de crypt(3).
const cryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// * HashPassword devuelve la contraseña como hash SHA-512 crypt ($6$...), el
// * formato que aceptan preseed (passwd/user-password-crypted) y kickstart.
// * Con 5000 rondas, el valor por defecto del estándar; la sal es aleatoria.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		salt[i] = cryptAlphabet[int(b)%len(cryptAlphabet)]
	}
	return sha512Crypt(password, string(salt), 5000)
}

// * sha512Crypt implementa el algoritmo SHA-512 crypt de Ulrich Drepper
// * (https://www.akkadia.org/drepper/SHA-crypt.txt).
func sha512Crypt(password, salt string, rounds int) (string, error) {
	if len(salt) == 0 || len(salt) > 16 || strings.ContainsAny(salt, "$:\n") {
		return "", errors.New("sal no válida")
	}
	key := []byte(password)
	saltBytes := []byte(salt)

	// * Digest B: clave + sal + clave.
	b := sha512.New()
	b.Write(key)
	b.Write(saltBytes)
	b.Write(key)
	digestB := b.Sum(nil)

	// * Digest A.
	a := sha512.New()
	a.Write(key)
	a.Write(saltBytes)
	for n := len(key); n > 0; n -= 64 {
		a.Write(digestB[:min(n, 64)])
	}
	for n := len(key); n > 0; n >>= 1 {
		if n&1 != 0 {
			a.Write(digestB)
		} else {
			a.Write(key)
		}
	}
	digestA := a.Sum(nil)

	// * Secuencia P: la clave repetida hasta su longitud.
	dp := sha512.New()
	for range key {
		dp.Write(key)
	}
	p := repeatTo(dp.Sum(nil), len(key))

	// * Secuencia S: la sal repetida 16 + A[0] veces.
	ds := sha512.New()
	for i := 0; i < 16+int(digestA[0]); i++ {
		ds.Write(saltBytes)
	}
	s := repeatTo(ds.Sum(nil), len(saltBytes))

	// * Rondas.
	digest := digestA
	for i := 0; i < rounds; i++ {
		c := sha512.New()
		if i&1 != 0 {
			c.Write(p)
		} else {
			c.Write(digest)
		}
		if i%3 != 0 {
			c.Write(s)
		}
		if i%7 != 0 {
			c.Write(p)
		}
		if i&1 != 0 {
			c.Write(digest)
		} else {
			c.Write(p)
		}
		digest = c.Sum(nil)
	}

	// * Codificación final con el orden de bytes del estándar.
	order := [][3]int{
		{0, 21, 42}, {22, 43, 1}, {44, 2, 23}, {3, 24, 45}, {25, 46, 4}, {47, 5, 26}, {6, 27, 48}, {28, 49, 7},
		{50, 8, 29}, {9, 30, 51}, {31, 52, 10}, {53, 11, 32}, {12, 33, 54}, {34, 55, 13}, {56, 14, 35}, {15, 36, 57},
		{37, 58, 16}, {59, 17, 38}, {18, 39, 60}, {40, 61, 19}, {62, 20, 41},
	}
	var out strings.Builder
	out.WriteString("$6$")
	if rounds != 5000 {
		out.WriteString("rounds=" + strconv.Itoa(rounds) + "$")
	}
	out.WriteString(salt + "$")
	for _, o := range order {
		out.WriteString(b64(digest[o[0]], digest[o[1]], digest[o[2]], 4))
	}
	out.WriteString(b64(0, 0, digest[63], 2))
	return out.String(), nil
}

// * repeatTo repite un bloque hasta tener n bytes.
func repeatTo(block []byte, n int) []byte {
	out := make([]byte, 0, n)
	for len(out) < n {
		out = append(out, block[:min(n-len(out), len(block))]...)
	}
	return out
}

// * b64 codifica tres bytes en n caracteres del alfabeto de crypt.
func b64(b2, b1, b0 byte, n int) string {
	value := uint32(b2)<<16 | uint32(b1)<<8 | uint32(b0)
	out := make([]byte, 0, n)
	for ; n > 0; n-- {
		out = append(out, cryptAlphabet[value&0x3f])
		value >>= 6
	}
	return string(out)
}
