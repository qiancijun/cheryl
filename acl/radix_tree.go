package acl

import (
	"strings"

	"com.cheryl/cheryl/logger"
)

const (
	NO_MISMATCH = -1
)

type RadixTree struct {
	root *Node
}

func NewRadixTree() *RadixTree {
	return &RadixTree{
		root: NewNode(false),
	}
}

func (tree *RadixTree) Insert(word string) {
	current := tree.root
	currIndex := 0
	for currIndex < len(word) {
		transitionChar := word[currIndex]
		currentEdge, has := current.GetTransition(transitionChar)
		currStr := word[currIndex:]

		if !has {
			current.edges[transitionChar] = NewEdge(currStr)
			break
		}

		splitIndex := getFirstMismatchLetter(currStr, currentEdge.label)
		if splitIndex == NO_MISMATCH {
			if len(currStr) == len(currentEdge.label) {
				currentEdge.next.isLeaf = true
				break
			} else if len(currStr) < len(currentEdge.label) {
				suffix := currentEdge.label[len(currStr):]
				currentEdge.label = currStr
				newNext := NewNode(true)
				afterNewNext := currentEdge.next
				currentEdge.next = newNext
				newNext.AddEdge(suffix, afterNewNext)
				break
			} else {
				splitIndex = len(currentEdge.label)
			}
		} else {
			suffix := currentEdge.label[splitIndex:]
			currentEdge.label = currentEdge.label[0:splitIndex]
			prevNext := currentEdge.next
			currentEdge.next = NewNode(false)
			currentEdge.next.AddEdge(suffix, prevNext)
		}

		current = currentEdge.next
		currIndex += splitIndex
	}
}

func getFirstMismatchLetter(word string, edgeWord string) int {
	length := min(len(word), len(edgeWord))
	for i := 1; i < length; i++ {
		if word[i] != edgeWord[i] {
			return i
		}
	}
	return NO_MISMATCH
}

func (tree *RadixTree) Search(word string) bool {
	current := tree.root
	currIndex := 0
	for currIndex < len(word) {
		transitionChar := word[currIndex]
		edge, has := current.GetTransition(transitionChar)
		if !has {
			return false
		}
		currSubstring := word[currIndex:]
		if !strings.HasPrefix(currSubstring, edge.label) {
			return false
		}
		currIndex += len(edge.label)
		current = edge.next
	}
	return current.isLeaf
}

func (tree *RadixTree) IPV4(ip string) bool {
	current := tree.root
	currIndex := 0
	for currIndex < len(ip) {
		transitionChar := ip[currIndex]
		edge, has := current.GetTransition(transitionChar)
		if !has {
			return false
		}
		if current.isLeaf {
			return true
		}
		currSubstring := ip[currIndex:]
		if !strings.HasPrefix(currSubstring, edge.label) {
			return false
		}
		currIndex += len(edge.label)
		current = edge.next
	}
	return current.isLeaf
}

func (tree *RadixTree) Delete(word string) {
	tree.root = deleteWord(tree.root, tree.root, word)
}

func deleteWord(root *Node, current *Node, word string) *Node {
	if word == "" {
		if len(current.edges) == 0 && current != nil {
			return nil
		}
		current.isLeaf = false
		return current
	}

	transitionChar := word[0]
	edge, has := current.GetTransition(transitionChar)
	if !has || !strings.HasPrefix(word, edge.label) {
		return current
	}
	deleted := deleteWord(root, edge.next, word[len(edge.label):])
	if deleted == nil {
		delete(current.edges, transitionChar)
		if current.TotalEdges() == 0 && !current.isLeaf && current != root {
			return nil
		}
	} else if deleted.TotalEdges() == 1 && !deleted.isLeaf {
		delete(current.edges, transitionChar)
		for _, afterDeleted := range deleted.edges {
			current.AddEdge(edge.label + afterDeleted.label, afterDeleted.next)
		}
	}
	return current
}

func (tree *RadixTree) PrintAllWords() {
	printAllWords(*tree.root, "")
}

func printAllWords(current Node, result string) {
	if current.isLeaf {
		logger.Debug(result)
	}
	for _, edge := range current.edges {
		printAllWords(*edge.next, result + edge.label)
	} 
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}
func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}