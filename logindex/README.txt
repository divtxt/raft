
                    lastCompacted       lastApplied         commitIndex     indexOfLastEntry
                    -------------       -----------         -----------     ----------------

Log                 Setter                                                  Setter

LogTail             Listener                                                (indirect setter)
 *Log


PCM                                                         Setter
 *LogTail


StateMachine                            Setter



Committer                               (indirect setter)   Listener        Listener
 *LogTail
 *StateMachine


Snapshotter/Compactor
 *Log/LogImpl                           Listener


