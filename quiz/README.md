# quiz


Q: Given a list of words like https://github.com/NodePrime/quiz/blob/master/word.list find the longest compound-word in the list, which is also a concatenation of other sub-words that exist in the list. The program should allow the user to input different data. The finished solution shouldn't take more than one hour. Any programming language can be used, but Go is preferred.

A: after doing 1st version code1.go, by checking sample data file, apparently length of words are small while number of words can be huge. so sub-parts of a word is much smaller combination space to search. so add code2.go.

following these steps:
   1> go to your $GOPATH/src
   2> git clone http://github.com/yglcode/quiz
   3> cd quiz/
   4> go run code1.go word.list or go run code2.go word.list

