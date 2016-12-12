#!/bin/bash
# minify -o out.html in.html
$1

echo "Moving all files to min-directory..."
cp -r statics/ min-statics/
cp -r templates/ min-templates/

# These don't minify well, will adjust
echo "Minfying templates..."
# minify -r -o min-templates templates/
echo "Minfying javascript..."
if [ "$1" == "closure" ] # Closure compile by google
	then
	for filename in statics/js/*; do
		echo "  Minifying ${filename}..."
		java -jar closure/compiler.jar  --js_output_file=min-${filename} ${filename} #--compilation_level=ADVANCED
	done
else
	minify -r -o min-statics/js statics/js/
fi

echo "Minfying css..."
minify -o min-statics/css/app.css statics/css/app.css
minify -o min-statics/css/other.css statics/css/other.css

echo "Done"