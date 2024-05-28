package main

import (
	"bytes"
	"math/big"
)

// Will return true if KeyA is located after KeyB
func compareKeys(keyA [20]byte, keyB [20]byte) bool {
	return bytes.Compare(keyA[:], keyB[:]) == 1
}

// Will return true if KeyA is equal to KeyB
func sameKey(keyA [20]byte, keyB [20]byte) bool {
	return bytes.Equal(keyA[:], keyB[:])
}

// will return true if key is between leftBound and rightBound (leftBound and rightBound included)
func keyInBetweenExclusive(leftBound [20]byte, rightBound [20]byte, key [20]byte) bool {
	return ((compareKeys(leftBound, rightBound) || sameKey(leftBound, rightBound)) && (compareKeys(key, leftBound) || compareKeys(rightBound, key))) || (compareKeys(rightBound, leftBound) && compareKeys(rightBound, key) && compareKeys(key, leftBound))
}

// will return true if key is between leftBound and rightBound or key is equal to rightBound
func keyInBetweenRightInclusive(leftBound [20]byte, rightBound [20]byte, key [20]byte) bool {
	return keyInBetweenExclusive(leftBound, rightBound, key) || sameKey(key, rightBound)
}

// add an integer to the identifier
// arguments: index as the interger to add
//
//	identifier as the hash number, made of byte
//
// returns the sum of the index and the identifier
func keyToLook(index int, identifier [20]byte) [20]byte {
	tmpResult := new(big.Int).SetBytes(identifier[:])
	two := big.NewInt(2)
	power := new(big.Int).Exp(two, big.NewInt(int64(index)), nil)

	tmpResult.Add(tmpResult, power)
	mask := new(big.Int).Exp(two, big.NewInt(8*20), nil)
	tmpResult.Mod(tmpResult, mask)

	resultBytes := tmpResult.Bytes()

	// Ensure the result has the correct length (20 bytes)
	if len(resultBytes) > 20 {
		resultBytes = resultBytes[len(resultBytes)-20:]
	} else {
		resultBytes = append(make([]byte, 20-len(resultBytes)), resultBytes...)
	}

	var result [20]byte
	copy(result[:], resultBytes)
	return result
}
