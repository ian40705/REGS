set shell := ["bash", "-uc"]
# (6) [6, 2, 1, 5, 3, 4]
usage:
  @just --list

template id:
  cmake -S "114FinalQ{{id}}/template" -B ".online-judge/{{id}}-template" -G Ninja -D CMAKE_EXPORT_COMPILE_COMMANDS=ON
  cmake --build ".online-judge/{{id}}-template"
  mkdir -p "114FinalQ{{id}}/online-judge-template"
  ctest --test-dir ".online-judge/{{id}}-template" -V
  for i in {1..5}; do echo -e "\n"; ".online-judge/{{id}}-template/case${i}" > "114FinalQ{{id}}/online-judge/case${i}" ; done
sol id:
  cmake -S "114FinalQ{{id}}/solution" -B ".online-judge/{{id}}-solution" -G Ninja -D CMAKE_EXPORT_COMPILE_COMMANDS=ON
  cmake --build ".online-judge/{{id}}-solution"
  mkdir -p "114FinalQ{{id}}/online-judge"
  ctest -Q --test-dir ".online-judge/{{id}}-solution" --output-junit "$(pwd)/114FinalQ{{id}}/online-judge/result.xml"
  for i in {1..5}; do echo -e "\n"; ".online-judge/{{id}}-solution/case${i}" > "114FinalQ{{id}}/online-judge/case${i}" ; done

archive:
  for dir in $(find . -name "*Q00*" -type d | sort); do \
    cd $dir ; zip -9rv $dir.zip ./* ; mv $dir.zip ../docs ; cd .. ; \
  done
copy-test:
  find . -name "test.h" -delete
  for dir in $(find . -name "solution" -o -name "template"); do \
    cp ./docs/catch.h "$dir/test.h"; \
  done
  for i in {1..6}; do just sol "00$i"; done