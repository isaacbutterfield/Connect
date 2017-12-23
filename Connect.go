package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	sWidth      int    = 7
	sHeight     uint64 = 6
	sMaskHeight        = sHeight + 1
	sMaxMoves          = sWidth * int(sHeight)
	sMaxTurns          = sMaxMoves / 2
	sMinScore          = -sMaxMoves / 2 + 3
	sDrawnGame  int    = 0
)

// Static assert that our grid fits in a dword
const _ uint = 64 - uint(sMaxTurns+sWidth)

var vNodesExplored int
var vColumnOrder [sWidth]int
var vMap map[uint64]int

func bottom(width uint64, height uint64) (i uint64) {
	i = 0
	if width != 0 {
		i = bottom(width - 1, height) | (1 << ((width - 1) * (height + 1)))
	}
	return
}

const bottomMask2 := bottom(uint(sWidth), sHeight)

// Position : game state
type Position struct {
	currentPlayerPosition uint64
	combinedMask          uint64
	moves                 int
}

func topMask(col int) uint64 {
	return 1 << (sHeight - 1) << (uint64(col) * sMaskHeight)
}

func bottomMask(col int) uint64 {
	return 1 << (uint64(col) * sMaskHeight)
}

func columnMask(col int) uint64 {
	return ((1 << sHeight) - 1) << (uint64(col) * sMaskHeight)
}

func isAlignment(pos uint64) bool {
	mask := pos & (pos >> sMaskHeight)             // horizontal groups of two
	if (mask & (mask >> (2 * sMaskHeight))) != 0 { // do we have any adjacent horizontal groups of two
		return true
	}

	mask = pos & (pos >> (sMaskHeight - 1))              // upper left to bottom right diagonal, groups of two
	if (mask & (mask >> (2 * (sMaskHeight - 1)))) != 0 { // two adjacent diagonal groups of two
		return true
	}

	mask = pos & (pos >> (sMaskHeight + 1))              // bottom left to upper right diagonal, groups of two
	if (mask & (mask >> (2 * (sMaskHeight + 1)))) != 0 { // two adjacent diagonal groups of two
		return true
	}

	mask = pos & (pos >> 1)        // vertical groups of two
	if (mask & (mask >> 2)) != 0 { // two adjacent vertical groups of two
		return true
	}

	return false
}

func computeWinningPosition(position uint64, mask uint64) uint64 {
	// vertical
	win := (position << 1) & (position << 2) & (position << 3)

	directions := []uint64{sHeight + 1, sHeight, sHeight + 2}

	for _, direction := range directions {
		pos := (position << direction) & (position << (2 * direction))
		win |= pos & (position << (3 * direction))
		win |= pos & (position >> direction)
		pos = (position >> direction) & (position >> (2 * direction))
		win |= pos & (position << direction)
		win |= pos & (position >> (3 * direction))
	}

	return win & (boardMask ^ mask)
}

func (p Position) isLegal(col int) bool {
	return (p.combinedMask & topMask(col)) == 0
}

func (p *Position) play(col int) {
	p.currentPlayerPosition ^= p.combinedMask            // invert current player for next turn
	p.combinedMask |= (p.combinedMask + bottomMask(col)) // make move for the old player
	p.moves++
}

func (p *Position) playSequence(seq string) error {
	for i := 0; i < len(seq); i++ {
		col, err := strconv.Atoi(seq[i : i+1])
		col--
		if err != nil {
			return err
		}
		if col < 0 || col >= int(sWidth) {
			return errors.New("looks like we have a bad column value")
		}
		if !p.isLegal(col) {
			return errors.New("looks like that isn't a valid move")
		}
		if p.isWinningMove(col) {
			return errors.New("sequences should not contain winning moves")
		}
		p.play(col)
	}
	return nil
}

func (p Position) isWinningMove(col int) bool {
	pos := p.currentPlayerPosition
	pos |= ((p.combinedMask + bottomMask(col)) & columnMask(col))
	return isAlignment(pos)
}

func (p Position) key() uint64 {
	return p.currentPlayerPosition + p.combinedMask
}

func negamax(p Position, alpha int, beta int) (int, error) {
	if alpha >= beta {
		return -1, errors.New("alpha should always be smaller than beta")
	}
	vNodesExplored++

	if p.moves == sMaxMoves { // do we have a drawn game?
		return sDrawnGame, nil
	}

	for i := 0; i < int(sWidth); i++ { // look for a winning move
		if p.isLegal(i) && p.isWinningMove(i) {
			return (int(sMaxMoves) + 1 - p.moves) / 2, nil
		}
	}

	max := (sMaxMoves + 1 - p.moves) / 2 // upper bound of our score as we cannot win immediately
	if val := vMap[p.key()]; val != 0 {
		max = val + sMinScore - 1
	}

	if beta > max {
		beta = max
		if alpha >= beta {
			return beta, nil
		}
	}

	for i := 0; i < sWidth; i++ {
		if p.isLegal(vColumnOrder[i]) {
			pNext := p
			pNext.play(vColumnOrder[i])
			score, err := negamax(pNext, -beta, -alpha)
			if err != nil {
				return -1, err
			}
			score = -score
			if score >= beta {
				return score, nil
			}
			if score > alpha {
				alpha = score
			}
		}
	}

	vMap[p.key()] = alpha - sMinScore + 1
	return alpha, nil
}

func solve(p Position, weak bool) (int, error) {
	vNodesExplored = 0
	vMap = make(map[uint64]int)
	for i := range vColumnOrder {
		vColumnOrder[i] = sWidth/2 + ((1 - 2*(i%2)) * (i + 1) / 2)
	}

	min := -(sMaxMoves - p.moves) / 2
	max := (sMaxMoves + 1 - p.moves) / 2
	if weak {
		min = -1
		max = 1
	}

	for ; min < max; {
		med := min + (max - min) / 2
		if (med <= 0) && (min / 2 < med) {
			med = min / 2
		} else if (med >= 0) && (max / 2 > med) {
			med = max / 2
		}
		r, err := negamax(p, med, med + 1)
		if err != nil {
			return -1, err
		}
		if r <= med {
			max = r
		} else {
			min = r
		}		
	}
	return min, nil
}

func main() {
	filePtr := flag.String("file", "test.txt", "file with play sequences")
	boolPtr := flag.Bool("weak", false, "enable weak solving")
	flag.Parse()

	file, err := os.Open(*filePtr)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// fmt.Println("line: ", scanner.Text())
		p := Position{}
		err = p.playSequence(strings.Split(scanner.Text(), " ")[0])
		if err != nil {
			log.Fatal(err)
		}
		start := time.Now()
		score, err := solve(p, *boolPtr)
		if err != nil {
			log.Fatal(err)
		}
		end := time.Now()
		fmt.Println(scanner.Text(), score, vNodesExplored, end.Sub(start).Nanoseconds()/1e6)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
