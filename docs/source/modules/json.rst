:code:`json`
=================

   The json module provides functions for encoding and decoding Starlark
   values to/from JSON.

.. py:module:: json

.. py:function:: encode(x)

   The encode function converts x to JSON by cases:
   - A Starlark value that implements Go's standard json.Marshal
     interface defines its own JSON encoding.
   - None, True, and False are converted to null, true, and false, respectively.
   - Starlark int values, no matter how large, are encoded as decimal integers.
     Some decoders may not be able to decode very large integers.
   - Starlark float values are encoded using decimal point notation,
     even if the value is an integer.
     It is an error to encode a non-finite floating-point value.
   - Starlark strings are encoded as JSON strings, using UTF-16 escapes.
   - a Starlark IterableMapping (e.g. dict) is encoded as a JSON object.
     It is an error if any key is not a string.
   - any other Starlark Iterable (e.g. list, tuple) is encoded as a JSON array.
   - a Starlark HasAttrs (e.g. struct) is encoded as a JSON object.
   It an application-defined type matches more than one the cases describe above,
   (e.g. it implements both Iterable and HasFields), the first case takes precedence.
   Encoding any other value yields an error.

   :param Any x: the value to encode
   :returns: the JSON-encoded value
   :rtype: str

.. py:function:: indent(x, prefix=None, indent=None)

   The indent function pretty-prints a valid JSON encoding,
   and returns a string containing the indented form.

   :param Any x: the value to encode
   :param str prefix: the prefix to prepend to each line of output, if any
   :param str indent: the unit of indentation.
   :returns: the JSON-encoded value
   :rtype: str

.. py:function:: decode(x)

   Returns the Starlark value denoted by a JSON string.
   - Numbers are parsed as int or float, depending on whether they
     contain a decimal point.
   - JSON objects are parsed as new unfrozen Starlark dicts.
   - JSON arrays are parsed as new unfrozen Starlark lists.
   Decoding fails if x is not a valid JSON string.

   :param str x: the string to decode
   :returns: the decoded value
   :rtype: Any

.. py:function:: decode_all(x)

   Returns a list of Starlark values denoted by a string that contains a
   sequence of JSON values. Decoding fails if x is not a sequence of
   valid JSON values.

   :param str x: the string to decode
   :returns: the decoded values
   :rtype: List[Any]
