:code:`cli`
=================


   CLI-only builtin functions.
   

.. py:module:: cli




.. py:function:: depends(label_or_target)

   Returns the transitive closure of targets depended on by the given
   target.

   :param label_or_target: the label or target in question.
   :returns: the target's transitive dependency closure.
   :rtype: List[str]
   

.. py:function:: what_depends(label_or_target)

   Returns the transitive closure of target that depend on the given target.

   :param label_or_target: the label or target in question.
   :returns: the target's transitive dependent closure.
   :rtype: List[str]
   


