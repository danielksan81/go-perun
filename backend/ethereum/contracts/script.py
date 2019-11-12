with open("structdefinition.txt", "rt") as struct:
	definition = struct.read()
	struct.close()
	with open("../adjudicator/Adjudicator.go", "rt") as fin:
	    data = fin.read().replace(definition, ' ', 1)
	    fin.close()
	    with open("../adjudicator/Adjudicator.go", "wt") as fout:
	    	fout.write(data)
	    	fout.close()