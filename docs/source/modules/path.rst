:code:`path`
=================


    The path module provides utility functions for manipulating host-specific
    paths. This module uses either forward- or backwards-facing slashes for
    separating path components, depending on the host operating system.
    

.. py:module:: path



.. py:attribute:: sep

        The host-specific path separator.
        


.. py:function:: is_abs(path)

    Returns True if path is absolute.
    

.. py:function:: abs(path)

    Returns an absolute representation of path. If the path is not absolute
    it will be joined with the current working directory (usually the
    directory containing the root module on the stack) to turn it into an
    absolute path.
    

.. py:function:: base(path)

    Returns the last element of path. Trailing path separators are removed
    before extracting the last element. If the path is empty, Base returns
    ".". If the path consists entirely of separators, Base returns a single
    separator.
    

.. py:function:: dir(path)

    Returns all but the last element of path. If the path is empty, dir
    returns ".". If the path consists entirely of separators, dir returns a
    single separator. The returned path does not end in a separator unless
    it is the root directory.
    

.. py:function:: join(components)

    Joins any number of path elements into a single path, separating them
    with a host-specific separator. Empty elements are ignored.
    

.. py:function:: split(path)

    Splits path immediately following the final separator, separating it into
    a directory and file name component. If there is no separator in path,
    split returns an empty dir and file set to path.
    

.. py:function:: splitext(path)

    Splits the pathname path into a pair (root, ext) such that
    root + ext == path, and ext is empty or begins with a period and contains
    at most one period. Leading periods on the basename are ignored;
    splitext('.cshrc') returns ('.cshrc', '').
    


